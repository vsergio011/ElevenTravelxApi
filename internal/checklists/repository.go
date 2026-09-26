package checklists

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository interface {
	CreateTask(ctx context.Context, planningID uuid.UUID, actorUserID uuid.UUID, input CreateChecklistTaskInput) (ChecklistTask, error)
	GetTaskByID(ctx context.Context, planningID uuid.UUID, taskID uuid.UUID) (ChecklistTask, error)
	ListTasks(ctx context.Context, planningID uuid.UUID, filters ChecklistTaskFilters) ([]ChecklistTask, error)
	UpdateTask(ctx context.Context, planningID uuid.UUID, taskID uuid.UUID, input UpdateChecklistTaskInput) (ChecklistTask, error)
	DeleteTask(ctx context.Context, planningID uuid.UUID, taskID uuid.UUID) error
	AddCompletion(ctx context.Context, taskID uuid.UUID, userID uuid.UUID) error
	RemoveCompletion(ctx context.Context, taskID uuid.UUID, userID uuid.UUID) error
	ClearCompletions(ctx context.Context, taskID uuid.UUID) error
}

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (r *PostgresRepository) CreateTask(ctx context.Context, planningID uuid.UUID, actorUserID uuid.UUID, input CreateChecklistTaskInput) (ChecklistTask, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return ChecklistTask{}, err
	}
	defer tx.Rollback(ctx)

	var task ChecklistTask
	err = tx.QueryRow(ctx, `
		INSERT INTO public.checklist_tasks (
			id, planning_id, created_by_user_id, title, type, priority, assignment_mode, due_date
		)
		VALUES (gen_random_uuid(), $1, $2, $3, $4, $5, $6, $7)
		RETURNING id, planning_id, created_by_user_id, title, type, priority, assignment_mode, due_date, created_at, updated_at
	`, planningID, actorUserID, input.Title, input.Type, input.Priority, input.AssignmentMode, input.DueDate).Scan(
		&task.ID,
		&task.PlanningID,
		&task.CreatedByUserID,
		&task.Title,
		&task.Type,
		&task.Priority,
		&task.AssignmentMode,
		&task.DueDate,
		&task.CreatedAt,
		&task.UpdatedAt,
	)
	if err != nil {
		return ChecklistTask{}, err
	}

	if err := replaceAssigneesTx(ctx, tx, task.ID, input.AssignedUserIDs); err != nil {
		return ChecklistTask{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return ChecklistTask{}, err
	}

	task.AssignedUserIDs = input.AssignedUserIDs
	task.CompletedByUserIDs = []uuid.UUID{}

	return task, nil
}

func (r *PostgresRepository) GetTaskByID(ctx context.Context, planningID uuid.UUID, taskID uuid.UUID) (ChecklistTask, error) {
	task, err := scanTask(r.pool.QueryRow(ctx, `
		SELECT id, planning_id, created_by_user_id, title, type, priority, assignment_mode, due_date, created_at, updated_at
		FROM public.checklist_tasks
		WHERE id = $1 AND planning_id = $2
	`, taskID, planningID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ChecklistTask{}, ErrTaskNotFound
		}

		return ChecklistTask{}, err
	}

	if task.AssignedUserIDs, err = r.listAssignees(ctx, task.ID); err != nil {
		return ChecklistTask{}, err
	}

	if task.CompletedByUserIDs, err = r.listCompletions(ctx, task.ID); err != nil {
		return ChecklistTask{}, err
	}

	return task, nil
}

func (r *PostgresRepository) ListTasks(ctx context.Context, planningID uuid.UUID, filters ChecklistTaskFilters) ([]ChecklistTask, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, planning_id, created_by_user_id, title, type, priority, assignment_mode, due_date, created_at, updated_at
		FROM public.checklist_tasks
		WHERE planning_id = $1 AND ($2::text IS NULL OR type = $2)
		ORDER BY created_at DESC
	`, planningID, filters.Type)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tasks := make([]ChecklistTask, 0)
	for rows.Next() {
		task, scanErr := scanTaskRow(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		tasks = append(tasks, task)
	}
	if rows.Err() != nil {
		return nil, rows.Err()
	}

	for index := range tasks {
		assignedUserIDs, assigneesErr := r.listAssignees(ctx, tasks[index].ID)
		if assigneesErr != nil {
			return nil, assigneesErr
		}
		tasks[index].AssignedUserIDs = assignedUserIDs

		completedByUserIDs, completionsErr := r.listCompletions(ctx, tasks[index].ID)
		if completionsErr != nil {
			return nil, completionsErr
		}
		tasks[index].CompletedByUserIDs = completedByUserIDs
	}

	return tasks, nil
}

func (r *PostgresRepository) UpdateTask(ctx context.Context, planningID uuid.UUID, taskID uuid.UUID, input UpdateChecklistTaskInput) (ChecklistTask, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return ChecklistTask{}, err
	}
	defer tx.Rollback(ctx)

	var dueDate any
	switch {
	case input.ClearDueDate:
		dueDate = nil
	case input.DueDate != nil:
		dueDate = *input.DueDate
	default:
		dueDate = nil
	}

	setDueDate := input.ClearDueDate || input.DueDate != nil

	task, err := scanTask(tx.QueryRow(ctx, `
		UPDATE public.checklist_tasks
		SET
			title = COALESCE($3, title),
			type = COALESCE($4, type),
			priority = COALESCE($5, priority),
			assignment_mode = COALESCE($6, assignment_mode),
			due_date = CASE WHEN $7 THEN $8::date ELSE due_date END,
			updated_at = now()
		WHERE id = $1 AND planning_id = $2
		RETURNING id, planning_id, created_by_user_id, title, type, priority, assignment_mode, due_date, created_at, updated_at
	`, taskID, planningID, input.Title, input.Type, input.Priority, input.AssignmentMode, setDueDate, dueDate))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ChecklistTask{}, ErrTaskNotFound
		}

		return ChecklistTask{}, err
	}

	if input.AssignedUserIDs != nil {
		if err := replaceAssigneesTx(ctx, tx, task.ID, *input.AssignedUserIDs); err != nil {
			return ChecklistTask{}, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return ChecklistTask{}, err
	}

	if task.AssignedUserIDs, err = r.listAssignees(ctx, task.ID); err != nil {
		return ChecklistTask{}, err
	}
	if task.CompletedByUserIDs, err = r.listCompletions(ctx, task.ID); err != nil {
		return ChecklistTask{}, err
	}

	return task, nil
}

func (r *PostgresRepository) DeleteTask(ctx context.Context, planningID uuid.UUID, taskID uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM public.checklist_tasks WHERE id = $1 AND planning_id = $2`, taskID, planningID)
	if err != nil {
		return err
	}

	if tag.RowsAffected() == 0 {
		return ErrTaskNotFound
	}

	return nil
}

func (r *PostgresRepository) AddCompletion(ctx context.Context, taskID uuid.UUID, userID uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO public.checklist_task_completions (task_id, user_id)
		VALUES ($1, $2)
		ON CONFLICT (task_id, user_id) DO NOTHING
	`, taskID, userID)

	return err
}

func (r *PostgresRepository) RemoveCompletion(ctx context.Context, taskID uuid.UUID, userID uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM public.checklist_task_completions WHERE task_id = $1 AND user_id = $2`, taskID, userID)

	return err
}

func (r *PostgresRepository) ClearCompletions(ctx context.Context, taskID uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM public.checklist_task_completions WHERE task_id = $1`, taskID)

	return err
}

func (r *PostgresRepository) listAssignees(ctx context.Context, taskID uuid.UUID) ([]uuid.UUID, error) {
	rows, err := r.pool.Query(ctx, `SELECT user_id FROM public.checklist_task_assignees WHERE task_id = $1 ORDER BY user_id`, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanUUIDs(rows)
}

func (r *PostgresRepository) listCompletions(ctx context.Context, taskID uuid.UUID) ([]uuid.UUID, error) {
	rows, err := r.pool.Query(ctx, `SELECT user_id FROM public.checklist_task_completions WHERE task_id = $1 ORDER BY user_id`, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanUUIDs(rows)
}

func replaceAssigneesTx(ctx context.Context, tx pgx.Tx, taskID uuid.UUID, userIDs []uuid.UUID) error {
	if _, err := tx.Exec(ctx, `DELETE FROM public.checklist_task_assignees WHERE task_id = $1`, taskID); err != nil {
		return err
	}

	for _, userID := range userIDs {
		if _, err := tx.Exec(ctx, `INSERT INTO public.checklist_task_assignees (task_id, user_id) VALUES ($1, $2)`, taskID, userID); err != nil {
			return err
		}
	}

	return nil
}

func scanUUIDs(rows pgx.Rows) ([]uuid.UUID, error) {
	ids := make([]uuid.UUID, 0)
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}

	return ids, rows.Err()
}

func scanTask(row pgx.Row) (ChecklistTask, error) {
	return scanTaskRow(row)
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanTaskRow(row rowScanner) (ChecklistTask, error) {
	var task ChecklistTask
	err := row.Scan(
		&task.ID,
		&task.PlanningID,
		&task.CreatedByUserID,
		&task.Title,
		&task.Type,
		&task.Priority,
		&task.AssignmentMode,
		&task.DueDate,
		&task.CreatedAt,
		&task.UpdatedAt,
	)

	return task, err
}
