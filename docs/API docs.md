# SecuMSG API Documentation

## Auth Service (`/v1` Internal Endpoints)

### `POST /v1/auth/register`

**Description:** Register a new user.

**Request Body:**

```json
{
  "email": "string",
  "username": "string",
  "password": "string"
  // ...other fields as required
}
```

**Response:**

```json
{
  "userId": {},
  "requiresEmailVerification" : true /* bool decided from server config */
}

```

**Errors:**

- `400 Bad Request` for invalid input
- `405 Method Not Allowed` for non-POST requests

---

### `POST /v1/auth/login`

**Description:** Authenticate a user and issue tokens.

**Request Body:**

```json
{
  "username": "string",
  "password": "string"
  // or "email": "string"
}
```

**Response:**

```json
{
  "accessToken": {
    /* user info */
  },
  "refreshToken": {
    /* access/refresh tokens */
  },
  "expiresIn" : 999 /* number decided by server config */
}
```

**Errors:**

- `400 Bad Request` for invalid input
- `401 Unauthorized` for failed login
- `405 Method Not Allowed` for non-POST requests

---

### `GET /healthz`

**Description:** Health check endpoint.

**Response:**

```
ok
```

**Errors:** None

---

## Gateway Service (External Endpoints)

### `GET /healthz`

**Description:** Health check endpoint.

**Response:**

```
ok
```

**Errors:** None

---

### `POST /auth/register`

**Description:** Proxy to `/v1/auth/register` on the auth service.

**Request Body:**

```json
{
  "email": "string",
  "username": "string",
  "password": "string"
  // ...other fields as required
}
```

**Response:**

```json
{
  "userId": {},
  "requiresEmailVerification" : true /* bool decided from server config */
}
```

---

### `POST /auth/login`

**Description:** Proxy to `/v1/auth/login` on the auth service.

**Request Body:**

```json
{
  "username": "string",
  "password": "string"
  // or "email": "string"
}
```

**Response:**

```json
{
  "accessToken": {
    /* user info */
  },
  "refreshToken": {
    /* access/refresh tokens */
  },
  "expiresIn" : 999 /* number decided by server config */
}
```

---

### `POST /auth/refresh`

### To be implemented soon

---

