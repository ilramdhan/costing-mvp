-- Rollback migration

DROP TRIGGER IF EXISTS trg_variant_cost_summaries_updated ON variant_cost_summaries;
DROP TRIGGER IF EXISTS trg_yarn_variants_updated ON yarn_variants;
DROP TRIGGER IF EXISTS trg_master_yarns_updated ON master_yarns;

DROP FUNCTION IF EXISTS update_updated_at();
DROP FUNCTION IF EXISTS get_current_rate(VARCHAR, DATE);

DROP TABLE IF EXISTS price_rates;
DROP TABLE IF EXISTS batch_jobs;
DROP TABLE IF EXISTS variant_cost_summaries;
DROP TABLE IF EXISTS variant_process_costs;
DROP TABLE IF EXISTS yarn_variants;
DROP TABLE IF EXISTS process_steps;
DROP TABLE IF EXISTS routing_templates;
DROP TABLE IF EXISTS process_masters;
DROP TABLE IF EXISTS master_yarns;
DROP TABLE IF EXISTS master_parameters;
DROP TABLE IF EXISTS parameter_groups;

DROP TYPE IF EXISTS job_type;
DROP TYPE IF EXISTS job_status;
