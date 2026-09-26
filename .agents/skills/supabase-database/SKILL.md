---
name: supabase-database
description: Use when a backend task requires inspecting, querying, validating or understanding the current Supabase PostgreSQL database schema or data. Use before modifying backend code or migrations when database state, relationships, constraints or existing records are relevant.
---

# Supabase Database Inspection

Supabase is the PostgreSQL database used by ElevenTravel.

Use the Supabase MCP to inspect the actual database when the backend task depends on:

- existing tables;
- columns;
- relationships;
- constraints;
- indexes;
- existing records;
- database functions;
- views;
- current schema;
- validating assumptions about persisted data.

## Before backend changes

When a backend change depends on database structure or persisted data:

1. Inspect the relevant database structure using the Supabase MCP.
2. Check existing tables and relationships.
3. Verify column names and types.
4. Check relevant constraints and indexes.
5. Inspect representative existing data when necessary.
6. Compare the database state with the existing Go models/repositories.
7. Only then implement the backend change.

Do not assume the database schema from:

- TypeScript types;
- Go structs;
- documentation;
- migration filenames;
- task descriptions.

The actual database should be treated as the source of truth for the current persisted state.

## Migrations

Before creating a migration:

1. Inspect the current schema.
2. Check whether the requested table/column/index/constraint already exists.
3. Inspect related tables and foreign keys.
4. Check existing migration conventions.
5. Create only the required migration.

Do not create migrations for changes that already exist in the database.

## Existing data

When changing a schema or backend behavior that depends on existing records:

- inspect representative data;
- identify nullable/empty/legacy states;
- verify assumptions before changing business logic.

Do not modify production data unless the task explicitly requires it.

## Security

Never expose:

- database credentials;
- service-role keys;
- access tokens;
- secrets.

Do not copy sensitive database contents into source code, tasks, skills or persistent project memory.

## Relationship with Codebase Memory

Use Codebase Memory to understand how the application accesses the database.

Use Supabase MCP to understand the actual database state.

Prefer:

Codebase Memory
→ Go model/repository/query
→ Supabase MCP
→ actual schema/data
→ implementation

Do not treat either tool as a replacement for the other.