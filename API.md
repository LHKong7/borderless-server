# API Documentation

## Base URL
```
http://localhost:8080/api/v1
```

## Authentication
Currently, the API does not implement authentication. In production, you should add JWT or OAuth2 authentication.

## Response Format
All responses are in JSON format with the following structure:
- Success responses include the requested data
- Error responses include an `error` field with the error message

## Health Checks

### GET /health
Basic health check endpoint.

**Response:**
```json
{
  "status": "healthy",
  "timestamp": "2024-01-01T00:00:00Z",
  "service": "borderless-coding-server"
}
```

### GET /health/ready
Readiness check that verifies all dependencies (PostgreSQL, Redis, MinIO).

**Response:**
```json
{
  "status": "ready",
  "timestamp": "2024-01-01T00:00:00Z",
  "checks": {
    "database": "healthy",
    "redis": "healthy",
    "minio": "healthy"
  }
}
```

### GET /health/live
Liveness check endpoint.

**Response:**
```json
{
  "status": "alive",
  "timestamp": "2024-01-01T00:00:00Z"
}
```

## Users

### POST /users
Create a new user.

**Request Body:**
```json
{
  "email": "user@example.com",
  "display_name": "John Doe",
  "password": "secure_password",
  "metadata": {
    "preferences": {
      "theme": "dark"
    }
  }
}
```

**Response:**
```json
{
  "message": "User created successfully",
  "user": {
    "id": "uuid",
    "email": "user@example.com",
    "display_name": "John Doe",
    "is_active": true,
    "metadata": {...},
    "created_at": "2024-01-01T00:00:00Z",
    "updated_at": "2024-01-01T00:00:00Z"
  }
}
```

### GET /users
List users with pagination.

**Query Parameters:**
- `offset` (int, default: 0): Number of records to skip
- `limit` (int, default: 10, max: 100): Number of records to return

**Response:**
```json
{
  "users": [...],
  "pagination": {
    "offset": 0,
    "limit": 10,
    "total": 100
  }
}
```

### GET /users/:id
Get a user by ID.

**Response:**
```json
{
  "user": {
    "id": "uuid",
    "email": "user@example.com",
    "display_name": "John Doe",
    "is_active": true,
    "metadata": {...},
    "created_at": "2024-01-01T00:00:00Z",
    "updated_at": "2024-01-01T00:00:00Z"
  }
}
```

### PUT /users/:id
Update a user.

**Request Body:**
```json
{
  "email": "newemail@example.com",
  "display_name": "Jane Doe",
  "is_active": true,
  "metadata": {...}
}
```

### DELETE /users/:id
Soft delete a user.

### GET /users/search
Search users by email or display name.

**Query Parameters:**
- `q` (string, required): Search query
- `limit` (int, default: 10, max: 50): Maximum results

### POST /users/:id/activate
Activate a user account.

### POST /users/:id/deactivate
Deactivate a user account.

### GET /users/:id/projects
Get all projects owned by a user.

### GET /users/:id/chat-sessions
Get all chat sessions for a user.

## Projects

### POST /users/:owner_id/projects
Create a new project.

**Request Body:**
```json
{
  "name": "My Project",
  "description": "Project description",
  "visibility": "private",
  "root_bucket": "borderless-coding",
  "root_prefix": "users/uuid/projects/uuid/",
  "storage_quota_bytes": 0,
  "meta": {
    "tags": ["web", "react"]
  }
}
```

**Response:**
```json
{
  "message": "Project created successfully",
  "project": {
    "id": "uuid",
    "owner_id": "uuid",
    "name": "My Project",
    "slug": "my-project",
    "description": "Project description",
    "visibility": "private",
    "root_bucket": "borderless-coding",
    "root_prefix": "users/uuid/projects/uuid/",
    "storage_quota_bytes": 0,
    "meta": {...},
    "version": 1,
    "created_at": "2024-01-01T00:00:00Z",
    "updated_at": "2024-01-01T00:00:00Z"
  }
}
```

### GET /projects
List projects with filtering and pagination.

**Query Parameters:**
- `offset` (int, default: 0): Number of records to skip
- `limit` (int, default: 10, max: 100): Number of records to return
- `owner_id` (uuid, optional): Filter by owner
- `visibility` (string, optional): Filter by visibility (private, unlisted, public)

### GET /projects/public
Get public projects only.

### GET /projects/:id
Get a project by ID.

### GET /users/:owner_id/projects/slug/:slug
Get a project by slug for a specific owner.

### PUT /projects/:id
Update a project.

**Request Body:**
```json
{
  "name": "Updated Project Name",
  "description": "Updated description",
  "visibility": "public",
  "storage_quota_bytes": 1000000000,
  "meta": {...}
}
```

### DELETE /projects/:id
Soft delete a project.

### PUT /projects/:id/visibility
Update project visibility.

**Request Body:**
```json
{
  "visibility": "public"
}
```

### PUT /projects/:id/storage-quota
Update project storage quota.

**Request Body:**
```json
{
  "storage_quota_bytes": 1000000000
}
```

### GET /projects/search
Search projects by name or description.

**Query Parameters:**
- `q` (string, required): Search query
- `limit` (int, default: 10, max: 50): Maximum results
- `owner_id` (uuid, optional): Filter by owner
- `visibility` (string, optional): Filter by visibility

### GET /projects/:id/chat-sessions
Get all chat sessions for a project.

## Chat Sessions

### POST /chat-sessions
Create a new chat session.

**Request Body:**
```json
{
  "user_id": "uuid",
  "project_id": "uuid",
  "title": "My Chat Session",
  "model_hint": "gpt-4",
  "meta": {
    "temperature": 0.7
  }
}
```

**Response:**
```json
{
  "message": "Chat session created successfully",
  "session": {
    "id": "uuid",
    "user_id": "uuid",
    "project_id": "uuid",
    "title": "My Chat Session",
    "model_hint": "gpt-4",
    "meta": {...},
    "created_at": "2024-01-01T00:00:00Z",
    "updated_at": "2024-01-01T00:00:00Z",
    "archived_at": null
  }
}
```

### GET /chat-sessions/:id
Get a chat session by ID.

### PUT /chat-sessions/:id
Update a chat session.

**Request Body:**
```json
{
  "title": "Updated Title",
  "model_hint": "gpt-3.5-turbo",
  "meta": {...}
}
```

### DELETE /chat-sessions/:id
Delete a chat session (hard delete).

### POST /chat-sessions/:id/archive
Archive a chat session.

### POST /chat-sessions/:id/unarchive
Unarchive a chat session.

### GET /users/:user_id/chat-sessions
List chat sessions for a user.

**Query Parameters:**
- `offset` (int, default: 0): Number of records to skip
- `limit` (int, default: 10, max: 100): Number of records to return
- `project_id` (uuid, optional): Filter by project
- `include_archived` (bool, default: false): Include archived sessions

### GET /users/:user_id/chat-sessions/recent
Get recent chat sessions for a user.

**Query Parameters:**
- `limit` (int, default: 10, max: 50): Maximum results

### GET /projects/:project_id/chat-sessions
Get chat sessions for a project.

**Query Parameters:**
- `include_archived` (bool, default: false): Include archived sessions

### GET /chat-sessions/:id/messages
Get messages for a chat session.

**Query Parameters:**
- `offset` (int, default: 0): Number of records to skip
- `limit` (int, default: 50, max: 100): Number of records to return
- `include_replies` (bool, default: false): Include reply relationships

### GET /chat-sessions/:id/with-messages
Get a chat session with its messages.

**Query Parameters:**
- `message_limit` (int, default: 50, max: 200): Maximum messages to return

## Chat Messages

### POST /chat-messages
Create a new chat message.

**Request Body:**
```json
{
  "session_id": "uuid",
  "sender": "user",
  "text": "Hello, how are you?",
  "content": {
    "blocks": [
      {
        "type": "text",
        "text": "Hello, how are you?"
      }
    ]
  },
  "tokens_used": 10,
  "tool_name": null,
  "reply_to": null
}
```

**Response:**
```json
{
  "message": {
    "id": "uuid",
    "session_id": "uuid",
    "sender": "user",
    "text": "Hello, how are you?",
    "content": {...},
    "tokens_used": 10,
    "tool_name": null,
    "reply_to": null,
    "created_at": "2024-01-01T00:00:00Z"
  }
}
```

### GET /chat-messages/:id
Get a chat message by ID.

### PUT /chat-messages/:id
Update a chat message.

**Request Body:**
```json
{
  "text": "Updated message text",
  "content": {...},
  "tokens_used": 15,
  "tool_name": "code_executor"
}
```

### DELETE /chat-messages/:id
Delete a chat message.

## Data Types

### Project Visibility
- `private`: Only the owner can see the project
- `unlisted`: Project is accessible via direct link but not listed publicly
- `public`: Project is visible to everyone

### Chat Sender
- `user`: Message from a user
- `assistant`: Message from an AI assistant
- `system`: System message
- `tool`: Message from a tool/function

### JSONB Fields
The `metadata` and `meta` fields are JSONB objects that can contain arbitrary JSON data for storing additional information.

## Error Responses

All error responses follow this format:
```json
{
  "error": "Error message describing what went wrong"
}
```

Common HTTP status codes:
- `200 OK`: Success
- `201 Created`: Resource created successfully
- `400 Bad Request`: Invalid request data
- `404 Not Found`: Resource not found
- `500 Internal Server Error`: Server error
