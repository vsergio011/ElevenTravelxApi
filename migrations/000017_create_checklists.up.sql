CREATE TABLE IF NOT EXISTS public.checklist_tasks (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  planning_id UUID NOT NULL REFERENCES public.plannings(id) ON DELETE CASCADE,
  created_by_user_id UUID NOT NULL REFERENCES auth.users(id) ON DELETE RESTRICT,
  title TEXT NOT NULL,
  type TEXT NOT NULL,
  priority TEXT NOT NULL DEFAULT 'medium',
  assignment_mode TEXT NOT NULL DEFAULT 'group',
  due_date DATE NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT checklist_tasks_priority_check CHECK (priority IN ('low', 'medium', 'high')),
  CONSTRAINT checklist_tasks_assignment_mode_check CHECK (assignment_mode IN ('group', 'individual'))
);

CREATE INDEX IF NOT EXISTS idx_checklist_tasks_planning_id ON public.checklist_tasks(planning_id);
CREATE INDEX IF NOT EXISTS idx_checklist_tasks_planning_id_type ON public.checklist_tasks(planning_id, type);

CREATE TRIGGER trg_checklist_tasks_set_updated_at
BEFORE UPDATE ON public.checklist_tasks
FOR EACH ROW
EXECUTE FUNCTION public.set_updated_at();

CREATE TABLE IF NOT EXISTS public.checklist_task_assignees (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  task_id UUID NOT NULL REFERENCES public.checklist_tasks(id) ON DELETE CASCADE,
  user_id UUID NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT checklist_task_assignees_unique UNIQUE (task_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_checklist_task_assignees_task_id ON public.checklist_task_assignees(task_id);

CREATE TABLE IF NOT EXISTS public.checklist_task_completions (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  task_id UUID NOT NULL REFERENCES public.checklist_tasks(id) ON DELETE CASCADE,
  user_id UUID NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
  completed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT checklist_task_completions_unique UNIQUE (task_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_checklist_task_completions_task_id ON public.checklist_task_completions(task_id);
