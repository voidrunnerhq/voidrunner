# Authentication API Documentation

This document describes the authentication endpoints available in the VoidRunner API.

## Overview

The VoidRunner API uses JWT (JSON Web Tokens) for authentication. The authentication system supports:

- User registration with email and password
- User login with email and password  
- JWT access token (15-minute expiration)
- JWT refresh token (7-day expiration)
- Token refresh without re-authentication
- Rate limiting on auth endpoints

## Base URL

```
http://localhost:8080/api/v1
```

## Authentication Endpoints

### 1. User Registration

**Endpoint:** `POST /auth/register`

**Rate Limit:** 5 requests per hour per IP

**Request Body:**
```json
{
  "email": "user@example.com",
  "password": "StrongPassword123!"
}
```

**Password Requirements:**
- Minimum 8 characters
- Maximum 128 characters
- At least one uppercase letter
- At least one lowercase letter
- At least one digit
- At least one special character

**Success Response (201):**
```json
{
  "access_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "refresh_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "token_type": "Bearer",
  "expires_in": 900,
  "user": {
    "id": "123e4567-e89b-12d3-a456-426614174000",
    "email": "user@example.com",
    "created_at": "2023-07-04T12:00:00Z",
    "updated_at": "2023-07-04T12:00:00Z"
  }
}
```

**Error Responses:**
- `400 Bad Request`: Invalid input format or validation errors
- `409 Conflict`: User with email already exists
- `429 Too Many Requests`: Rate limit exceeded

### 2. User Login

**Endpoint:** `POST /auth/login`

**Rate Limit:** 10 requests per hour per IP

**Request Body:**
```json
{
  "email": "user@example.com",
  "password": "StrongPassword123!"
}
```

**Success Response (200):**
```json
{
  "access_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "refresh_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "token_type": "Bearer",
  "expires_in": 900,
  "user": {
    "id": "123e4567-e89b-12d3-a456-426614174000",
    "email": "user@example.com",
    "created_at": "2023-07-04T12:00:00Z",
    "updated_at": "2023-07-04T12:00:00Z"
  }
}
```

**Error Responses:**
- `400 Bad Request`: Invalid input format
- `401 Unauthorized`: Invalid email or password
- `429 Too Many Requests`: Rate limit exceeded

### 3. Token Refresh

**Endpoint:** `POST /auth/refresh`

**Rate Limit:** 100 requests per hour per IP

**Request Body:**
```json
{
  "refresh_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
}
```

**Success Response (200):**
```json
{
  "access_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "refresh_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "token_type": "Bearer",
  "expires_in": 900,
  "user": {
    "id": "123e4567-e89b-12d3-a456-426614174000",
    "email": "user@example.com",
    "created_at": "2023-07-04T12:00:00Z",
    "updated_at": "2023-07-04T12:00:00Z"
  }
}
```

**Error Responses:**
- `400 Bad Request`: Invalid request format
- `401 Unauthorized`: Invalid or expired refresh token
- `429 Too Many Requests`: Rate limit exceeded

### 4. User Logout

**Endpoint:** `POST /auth/logout`

**Description:** In a JWT system, logout is typically handled client-side by removing tokens from storage. This endpoint returns a success message.

**Success Response (200):**
```json
{
  "message": "Successfully logged out"
}
```

### 5. Get Current User

**Endpoint:** `GET /auth/me`

**Authentication:** Required (Bearer token)

**Headers:**
```
Authorization: Bearer <access_token>
```

**Success Response (200):**
```json
{
  "user": {
    "id": "123e4567-e89b-12d3-a456-426614174000",
    "email": "user@example.com",
    "created_at": "2023-07-04T12:00:00Z",
    "updated_at": "2023-07-04T12:00:00Z"
  }
}
```

**Error Responses:**
- `401 Unauthorized`: Missing, invalid, or expired token

## Authentication Flow

### Initial Authentication
1. User registers with `POST /auth/register` or logs in with `POST /auth/login`
2. Server returns access token (15-minute expiration) and refresh token (7-day expiration)
3. Client stores both tokens securely

### Making Authenticated Requests
1. Include access token in Authorization header: `Authorization: Bearer <access_token>`
2. If access token is expired, use refresh token to get new tokens
3. Update stored tokens with new values

### Token Refresh
1. When access token expires, call `POST /auth/refresh` with refresh token
2. Server returns new access token and refresh token
3. Client updates stored tokens

### Logout
1. Call `POST /auth/logout` (optional, for logging purposes)
2. Remove tokens from client storage

## Error Handling

All endpoints return errors in the following format:

```json
{
  "error": "Error message description",
  "details": "Additional error details (optional)"
}
```

Common HTTP status codes:
- `400 Bad Request`: Invalid input or request format
- `401 Unauthorized`: Authentication required or failed
- `403 Forbidden`: Access denied
- `409 Conflict`: Resource already exists
- `429 Too Many Requests`: Rate limit exceeded
- `500 Internal Server Error`: Server error

## Rate Limiting

The authentication endpoints have the following rate limits:

| Endpoint | Limit | Window |
|----------|-------|--------|
| `/auth/register` | 5 requests | 1 hour |
| `/auth/login` | 10 requests | 1 hour |
| `/auth/refresh` | 100 requests | 1 hour |

Rate limits are applied per IP address. When the limit is exceeded, the server returns:

```json
{
  "error": "Rate limit exceeded",
  "retry_after": 3600
}
```

## Security Considerations

1. **HTTPS Only**: Use HTTPS in production to protect tokens in transit
2. **Token Storage**: Store tokens securely (secure cookies, encrypted storage)
3. **Token Expiration**: Access tokens expire in 15 minutes, refresh tokens in 7 days
4. **Secret Key**: Use a strong, random secret key (minimum 256 bits)
5. **Rate Limiting**: Authentication endpoints are rate-limited to prevent abuse
6. **Password Policy**: Strong password requirements enforced

## Environment Configuration

Required environment variables:

```bash
# JWT Configuration
JWT_SECRET_KEY=your-secret-key-change-in-production-256-bits-minimum
JWT_ACCESS_TOKEN_DURATION=15m
JWT_REFRESH_TOKEN_DURATION=168h
JWT_ISSUER=voidrunner
JWT_AUDIENCE=voidrunner-api
```

For a complete list of configuration options, see `.env.example`.