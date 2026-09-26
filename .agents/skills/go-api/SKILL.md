---
name: go-api
description: Use when creating or modifying Go REST API endpoints, handlers, services, repositories, DTOs, request/response contracts or API errors.
---

# Go API

API uses `/v1`.

Before creating an endpoint:

1. Find equivalent endpoints.
2. Follow existing route conventions.
3. Reuse request/response structures.
4. Reuse error handling.
5. Reuse pagination.
6. Reuse authorization.

Separate handler, service, repository, DTO and validation layers when consistent with the existing domain.

Do not invent API contracts.