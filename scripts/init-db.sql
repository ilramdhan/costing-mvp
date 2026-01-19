-- Initial database setup
-- This file is executed automatically when PostgreSQL container starts

-- Create extensions (may already exist from migration, but safe to run)
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pg_trgm";

-- Grant permissions
GRANT ALL PRIVILEGES ON DATABASE costing TO postgres;
