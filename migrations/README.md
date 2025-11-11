# Database Migrations

This directory contains database migration files for the Tundra project.

## Migration Workflow

### 1. Create a New Migration

To create a new migration based on your GORM models:

```bash
make migrate-create name=add_username_to_users
```

This will create two files:

- `migrations/YYYYMMDDHHMMSS_add_username_to_users.up.sql` - Forward migration
- `migrations/YYYYMMDDHHMMSS_add_username_to_users.down.sql` - Rollback migration

### 2. Edit Migration Files

After creating the migration files, edit them to add your SQL statements:

**Example `up.sql`:**

```sql
-- Migration: add_username_to_users
-- Add username column to users table

ALTER TABLE users ADD COLUMN username VARCHAR(255) UNIQUE NOT NULL;
CREATE INDEX idx_users_username ON users(username);
```

**Example `down.sql`:**

```sql
-- Rollback: add_username_to_users
-- Remove username column from users table

DROP INDEX IF EXISTS idx_users_username;
ALTER TABLE users DROP COLUMN username;
```

### 3. Run Migrations

Apply all pending migrations:

```bash
make migrate-up-all
```

Apply one migration at a time:

```bash
make migrate-up
```

### 4. Rollback Migrations

Rollback the last migration:

```bash
make migrate-down
```

Rollback all migrations (⚠️ WARNING: This will drop all tables):

```bash
make migrate-down-all
```

### 5. Check Migration Status

See which migrations have been applied:

```bash
make migrate-status
```

### 6. Refresh All Migrations

Drop all tables and reapply all migrations:

```bash
make migrate-refresh
```

## Migration Naming Conventions

Use descriptive names that indicate what the migration does:

- `create_users_table`
- `add_email_to_users`
- `remove_deprecated_column`
- `create_products_table`
- `add_indexes_to_orders`

## Tips for Writing Migrations

1. **Always test migrations on development first**
2. **Make migrations reversible** - Always write both up and down migrations
3. **One change per migration** - Keep migrations focused and atomic
4. **Use transactions** - Wrap DDL statements in transactions when possible
5. **Add indexes carefully** - Use `CONCURRENTLY` for production databases
6. **Document your changes** - Add comments explaining complex migrations

## GORM Model to SQL Mapping

When creating migrations from GORM models, here are the common mappings:

### GORM Tags to SQL:

- `gorm:"primaryKey"` → `PRIMARY KEY`
- `gorm:"uniqueIndex"` → `UNIQUE`
- `gorm:"not null"` → `NOT NULL`
- `gorm:"default:value"` → `DEFAULT value`
- `gorm:"index"` → Creates an index
- `gorm:"type:varchar(100)"` → Custom SQL type

### Example Model:

```go
type User struct {
    ID        uint           `gorm:"primaryKey"`
    Username  string         `gorm:"uniqueIndex;not null"`
    Email     string         `gorm:"uniqueIndex;not null"`
    Password  string         `gorm:"not null"`
    CreatedAt time.Time
    UpdatedAt time.Time
    DeletedAt gorm.DeletedAt `gorm:"index"`
}
```

### Equivalent SQL:

```sql
CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    username VARCHAR(255) NOT NULL UNIQUE,
    email VARCHAR(255) NOT NULL UNIQUE,
    password VARCHAR(255) NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMP
);

CREATE INDEX idx_users_deleted_at ON users(deleted_at);
CREATE INDEX idx_users_username ON users(username);
CREATE INDEX idx_users_email ON users(email);
```

## Troubleshooting

### Migration is stuck in "dirty" state

If a migration fails halfway through, it will be marked as "dirty". To fix:

1. Manually fix the database to a known state
2. Force the migration version:
   ```bash
   go run cmd/migrate/main.go -action=force <version_number>
   ```

### Need to skip a migration

You cannot skip migrations. Instead:

1. Roll back to before the problematic migration
2. Fix the migration
3. Run migrations again

### Need to edit an applied migration

Never edit migrations that have already been applied. Instead:

1. Create a new migration with the changes
2. Apply the new migration
