-- Textile Costing Engine - Database Schema
-- Optimized for high-performance bulk operations

-- Enable necessary extensions
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pg_trgm";

-- ============================================
-- PARAMETER MANAGEMENT
-- ============================================

-- Parameter groups for organization
CREATE TABLE parameter_groups (
    code VARCHAR(50) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Master parameter definitions
CREATE TABLE master_parameters (
    key VARCHAR(100) PRIMARY KEY,
    label VARCHAR(255) NOT NULL,
    data_type VARCHAR(20) NOT NULL DEFAULT 'float', -- float, int, bool, string
    default_value VARCHAR(255),
    group_code VARCHAR(50) REFERENCES parameter_groups(code),
    unit VARCHAR(50), -- kg, meter, kwh, etc.
    is_required BOOLEAN DEFAULT FALSE,
    sequence_order INT DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_master_params_group ON master_parameters(group_code);

-- ============================================
-- CATALOG DOMAIN
-- ============================================

-- Master Yarn (500K records expected)
CREATE TABLE master_yarns (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    code VARCHAR(100) NOT NULL UNIQUE,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    fixed_attrs JSONB DEFAULT '{}', -- 10 fixed parameters
    is_active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_master_yarns_code ON master_yarns(code);
CREATE INDEX idx_master_yarns_active ON master_yarns(is_active) WHERE is_active = TRUE;
CREATE INDEX idx_master_yarns_fixed_attrs ON master_yarns USING GIN (fixed_attrs);

-- ============================================
-- ENGINEERING DOMAIN
-- ============================================

-- Process Masters (e.g., Smelting, Spinning, Weaving, Dyeing, Finishing, Packing)
CREATE TABLE process_masters (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    code VARCHAR(50) NOT NULL UNIQUE,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    default_sequence INT DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Routing Templates (combinations of processes)
CREATE TABLE routing_templates (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(255) NOT NULL UNIQUE,
    description TEXT,
    is_active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Process Steps within a Routing (with formulas)
CREATE TABLE process_steps (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    routing_template_id UUID NOT NULL REFERENCES routing_templates(id) ON DELETE CASCADE,
    process_master_id UUID NOT NULL REFERENCES process_masters(id),
    sequence_order INT NOT NULL,
    formula_expression TEXT NOT NULL, -- e.g., "(electricity_kwh * rate_per_kwh) + (labor_hours * labor_rate)"
    description TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(routing_template_id, sequence_order)
);

CREATE INDEX idx_process_steps_routing ON process_steps(routing_template_id);
CREATE INDEX idx_process_steps_sequence ON process_steps(routing_template_id, sequence_order);

-- ============================================
-- COSTING DOMAIN
-- ============================================

-- Yarn Variants (250M records expected: 500K masters × 500 children)
CREATE TABLE yarn_variants (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    master_yarn_id UUID NOT NULL REFERENCES master_yarns(id) ON DELETE CASCADE,
    sku VARCHAR(100) NOT NULL UNIQUE,
    batch_no VARCHAR(100),
    routing_template_id UUID REFERENCES routing_templates(id),
    is_active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_yarn_variants_master ON yarn_variants(master_yarn_id);
CREATE INDEX idx_yarn_variants_routing ON yarn_variants(routing_template_id);
CREATE INDEX idx_yarn_variants_active ON yarn_variants(is_active) WHERE is_active = TRUE;
CREATE INDEX idx_yarn_variants_sku ON yarn_variants(sku);

-- Variant Process Costs (62.5B records expected: 250M variants × avg 5 processes × 250 params)
-- Using hash partitioning for better distribution
CREATE TABLE variant_process_costs (
    id UUID DEFAULT uuid_generate_v4(),
    yarn_variant_id UUID NOT NULL,
    process_step_id UUID NOT NULL,
    input_values JSONB DEFAULT '{}', -- 250 parameters as key-value
    calculated_cost DECIMAL(18, 6) DEFAULT 0,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    PRIMARY KEY (id, yarn_variant_id)
) PARTITION BY HASH (yarn_variant_id);

-- Create 16 partitions for better parallelism
CREATE TABLE variant_process_costs_p0 PARTITION OF variant_process_costs FOR VALUES WITH (MODULUS 16, REMAINDER 0);
CREATE TABLE variant_process_costs_p1 PARTITION OF variant_process_costs FOR VALUES WITH (MODULUS 16, REMAINDER 1);
CREATE TABLE variant_process_costs_p2 PARTITION OF variant_process_costs FOR VALUES WITH (MODULUS 16, REMAINDER 2);
CREATE TABLE variant_process_costs_p3 PARTITION OF variant_process_costs FOR VALUES WITH (MODULUS 16, REMAINDER 3);
CREATE TABLE variant_process_costs_p4 PARTITION OF variant_process_costs FOR VALUES WITH (MODULUS 16, REMAINDER 4);
CREATE TABLE variant_process_costs_p5 PARTITION OF variant_process_costs FOR VALUES WITH (MODULUS 16, REMAINDER 5);
CREATE TABLE variant_process_costs_p6 PARTITION OF variant_process_costs FOR VALUES WITH (MODULUS 16, REMAINDER 6);
CREATE TABLE variant_process_costs_p7 PARTITION OF variant_process_costs FOR VALUES WITH (MODULUS 16, REMAINDER 7);
CREATE TABLE variant_process_costs_p8 PARTITION OF variant_process_costs FOR VALUES WITH (MODULUS 16, REMAINDER 8);
CREATE TABLE variant_process_costs_p9 PARTITION OF variant_process_costs FOR VALUES WITH (MODULUS 16, REMAINDER 9);
CREATE TABLE variant_process_costs_p10 PARTITION OF variant_process_costs FOR VALUES WITH (MODULUS 16, REMAINDER 10);
CREATE TABLE variant_process_costs_p11 PARTITION OF variant_process_costs FOR VALUES WITH (MODULUS 16, REMAINDER 11);
CREATE TABLE variant_process_costs_p12 PARTITION OF variant_process_costs FOR VALUES WITH (MODULUS 16, REMAINDER 12);
CREATE TABLE variant_process_costs_p13 PARTITION OF variant_process_costs FOR VALUES WITH (MODULUS 16, REMAINDER 13);
CREATE TABLE variant_process_costs_p14 PARTITION OF variant_process_costs FOR VALUES WITH (MODULUS 16, REMAINDER 14);
CREATE TABLE variant_process_costs_p15 PARTITION OF variant_process_costs FOR VALUES WITH (MODULUS 16, REMAINDER 15);

-- Indexes on partitioned table
CREATE INDEX idx_vpc_variant ON variant_process_costs(yarn_variant_id);
CREATE INDEX idx_vpc_step ON variant_process_costs(process_step_id);
CREATE INDEX idx_vpc_input_values ON variant_process_costs USING GIN (input_values);

-- Cost Summary (Read Model - CQRS pattern)
CREATE TABLE variant_cost_summaries (
    yarn_variant_id UUID PRIMARY KEY,
    total_material_cost DECIMAL(18, 6) DEFAULT 0,
    total_process_cost DECIMAL(18, 6) DEFAULT 0,
    total_overhead DECIMAL(18, 6) DEFAULT 0,
    grand_total DECIMAL(18, 6) DEFAULT 0,
    last_recalculated_at TIMESTAMP WITH TIME ZONE,
    version_hash VARCHAR(64), -- SHA256 hash of input params for change detection
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- ============================================
-- JOB MANAGEMENT
-- ============================================

CREATE TYPE job_status AS ENUM ('PENDING', 'RUNNING', 'COMPLETED', 'FAILED', 'CANCELLED');
CREATE TYPE job_type AS ENUM ('RECALCULATE_ALL', 'RECALCULATE_MASTER', 'RECALCULATE_VARIANT', 'IMPORT_DATA', 'EXPORT_DATA');

CREATE TABLE batch_jobs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    job_type job_type NOT NULL,
    status job_status NOT NULL DEFAULT 'PENDING',
    total_records BIGINT DEFAULT 0,
    processed_records BIGINT DEFAULT 0,
    failed_records BIGINT DEFAULT 0,
    metadata JSONB DEFAULT '{}', -- Additional job info
    error_message TEXT,
    started_at TIMESTAMP WITH TIME ZONE,
    finished_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_batch_jobs_status ON batch_jobs(status);
CREATE INDEX idx_batch_jobs_type ON batch_jobs(job_type);
CREATE INDEX idx_batch_jobs_created ON batch_jobs(created_at DESC);

-- ============================================
-- PRICING (Monthly Rates)
-- ============================================

CREATE TABLE price_rates (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    parameter_key VARCHAR(100) NOT NULL REFERENCES master_parameters(key),
    rate_value DECIMAL(18, 6) NOT NULL,
    effective_date DATE NOT NULL,
    expired_date DATE,
    notes TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(parameter_key, effective_date)
);

CREATE INDEX idx_price_rates_param ON price_rates(parameter_key);
CREATE INDEX idx_price_rates_effective ON price_rates(effective_date DESC);

-- ============================================
-- HELPER FUNCTIONS
-- ============================================

-- Function to get current price rate for a parameter
CREATE OR REPLACE FUNCTION get_current_rate(p_key VARCHAR, p_date DATE DEFAULT CURRENT_DATE)
RETURNS DECIMAL AS $$
BEGIN
    RETURN (
        SELECT rate_value
        FROM price_rates
        WHERE parameter_key = p_key
          AND effective_date <= p_date
          AND (expired_date IS NULL OR expired_date > p_date)
        ORDER BY effective_date DESC
        LIMIT 1
    );
END;
$$ LANGUAGE plpgsql STABLE;

-- Trigger to update updated_at timestamps
CREATE OR REPLACE FUNCTION update_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_master_yarns_updated
    BEFORE UPDATE ON master_yarns
    FOR EACH ROW EXECUTE FUNCTION update_updated_at();

CREATE TRIGGER trg_yarn_variants_updated
    BEFORE UPDATE ON yarn_variants
    FOR EACH ROW EXECUTE FUNCTION update_updated_at();

CREATE TRIGGER trg_variant_cost_summaries_updated
    BEFORE UPDATE ON variant_cost_summaries
    FOR EACH ROW EXECUTE FUNCTION update_updated_at();
