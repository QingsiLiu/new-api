-- RMB-native billing migration: legacy 500k quota/USD -> 100k quota/CNY.
-- Run with: psql -v ON_ERROR_STOP=1 -f scripts/migrate_quota_to_cny100k.sql
-- Required before production run: pg_dump backup and replica dry-run reconciliation.

\set ON_ERROR_STOP on

INSERT INTO options (key, value)
VALUES ('QuotaMigrationInProgress', 'true')
ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value;

DO $$
BEGIN
  IF EXISTS (
    SELECT 1 FROM options
    WHERE key = 'QuotaMigrationVersion'
      AND value = 'cny100k_20260703'
  ) THEN
    UPDATE options SET value = 'false' WHERE key = 'QuotaMigrationInProgress';
    RAISE NOTICE 'Quota ledger already migrated to cny100k_20260703, skipping.';
    RETURN;
  END IF;
END $$;

BEGIN;

DO $$
DECLARE
  target record;
BEGIN
  FOR target IN
    SELECT *
    FROM (VALUES
      ('users', 'quota'),
      ('users', 'used_quota'),
      ('users', 'aff_quota'),
      ('users', 'aff_history'),
      ('tokens', 'remain_quota'),
      ('tokens', 'used_quota'),
      ('logs', 'quota'),
      ('redemptions', 'quota'),
      ('channels', 'used_quota'),
      ('tasks', 'quota'),
      ('quota_data', 'quota'),
      ('subscription_plans', 'total_amount'),
      ('user_subscriptions', 'amount_total'),
      ('user_subscriptions', 'amount_used'),
      ('subscription_pre_consume_records', 'pre_consumed'),
      ('user_quota_change_records', 'delta'),
      ('user_quota_change_records', 'before_quota'),
      ('user_quota_change_records', 'after_quota'),
      ('checkins', 'quota_awarded'),
      ('midjourneys', 'quota')
    ) AS t(table_name, column_name)
  LOOP
    IF EXISTS (
      SELECT 1
      FROM information_schema.columns
      WHERE table_schema = 'public'
        AND table_name = target.table_name
        AND column_name = target.column_name
        AND data_type <> 'bigint'
    ) THEN
      EXECUTE format(
        'ALTER TABLE public.%I ALTER COLUMN %I TYPE bigint USING %I::bigint',
        target.table_name,
        target.column_name,
        target.column_name
      );
    END IF;
  END LOOP;
END $$;

DO $$
DECLARE
  migration_version text := 'cny100k_20260703';
BEGIN
  IF EXISTS (
    SELECT 1 FROM options
    WHERE key = 'QuotaMigrationVersion'
      AND value = migration_version
  ) THEN
    RAISE NOTICE 'Quota ledger already migrated to %, skipping.', migration_version;
    RETURN;
  END IF;

  IF to_regclass('public.users') IS NOT NULL THEN
    UPDATE users
    SET quota = floor(COALESCE(quota, 0)::numeric * 0.2)::bigint,
        used_quota = floor(COALESCE(used_quota, 0)::numeric * 0.2)::bigint,
        aff_quota = floor(COALESCE(aff_quota, 0)::numeric * 0.2)::bigint,
        aff_history = floor(COALESCE(aff_history, 0)::numeric * 0.2)::bigint;
  END IF;

  IF to_regclass('public.tokens') IS NOT NULL THEN
    UPDATE tokens
    SET remain_quota = floor(COALESCE(remain_quota, 0)::numeric * 0.2)::bigint,
        used_quota = floor(COALESCE(used_quota, 0)::numeric * 0.2)::bigint;
  END IF;

  IF to_regclass('public.logs') IS NOT NULL THEN
    UPDATE logs
    SET quota = floor(COALESCE(quota, 0)::numeric * 0.2)::bigint;
  END IF;

  IF to_regclass('public.redemptions') IS NOT NULL THEN
    UPDATE redemptions
    SET quota = floor(COALESCE(quota, 0)::numeric * 0.2)::bigint;
  END IF;

  IF to_regclass('public.channels') IS NOT NULL THEN
    UPDATE channels
    SET used_quota = floor(COALESCE(used_quota, 0)::numeric * 0.2)::bigint;
  END IF;

  IF to_regclass('public.tasks') IS NOT NULL THEN
    UPDATE tasks
    SET quota = floor(COALESCE(quota, 0)::numeric * 0.2)::bigint;
  END IF;

  IF to_regclass('public.quota_data') IS NOT NULL THEN
    UPDATE quota_data
    SET quota = floor(COALESCE(quota, 0)::numeric * 0.2)::bigint;
  END IF;

  IF to_regclass('public.subscription_plans') IS NOT NULL THEN
    UPDATE subscription_plans
    SET total_amount = floor(COALESCE(total_amount, 0)::numeric * 0.2)::bigint,
        currency = 'CNY';
  END IF;

  IF to_regclass('public.user_subscriptions') IS NOT NULL THEN
    UPDATE user_subscriptions
    SET amount_total = floor(COALESCE(amount_total, 0)::numeric * 0.2)::bigint,
        amount_used = floor(COALESCE(amount_used, 0)::numeric * 0.2)::bigint;
  END IF;

  IF to_regclass('public.subscription_pre_consume_records') IS NOT NULL THEN
    UPDATE subscription_pre_consume_records
    SET pre_consumed = floor(COALESCE(pre_consumed, 0)::numeric * 0.2)::bigint;
  END IF;

  IF to_regclass('public.user_quota_change_records') IS NOT NULL THEN
    UPDATE user_quota_change_records
    SET delta = floor(COALESCE(delta, 0)::numeric * 0.2)::bigint,
        before_quota = floor(COALESCE(before_quota, 0)::numeric * 0.2)::bigint,
        after_quota = floor(COALESCE(after_quota, 0)::numeric * 0.2)::bigint;
  END IF;

  IF to_regclass('public.checkins') IS NOT NULL THEN
    UPDATE checkins
    SET quota_awarded = floor(COALESCE(quota_awarded, 0)::numeric * 0.2)::bigint;
  END IF;

  IF to_regclass('public.midjourneys') IS NOT NULL THEN
    UPDATE midjourneys
    SET quota = floor(COALESCE(quota, 0)::numeric * 0.2)::bigint;
  END IF;

  INSERT INTO options (key, value)
  VALUES
    ('QuotaPerUnit', '100000'),
    ('QuotaPerCNY', '100000'),
	    ('USDExchangeRate', '1'),
	    ('Price', '1'),
	    ('StripeUnitPrice', '1'),
	    ('WaffoCurrency', 'CNY'),
	    ('theme.frontend', 'default')
	  ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value;

  INSERT INTO options (key, value)
  VALUES ('QuotaMigrationVersion', migration_version)
  ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value;

  INSERT INTO options (key, value)
  VALUES ('QuotaMigrationInProgress', 'false')
  ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value;
END $$;

COMMIT;
