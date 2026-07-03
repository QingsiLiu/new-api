-- RMB-native billing migration: legacy 500k quota/USD -> 100k quota/CNY.
--
-- Replica dry-run:
--   psql -v ON_ERROR_STOP=1 -v DRY_RUN=1 -f scripts/migrate_quota_to_cny100k.sql
--
-- Production run:
--   1. pg_dump backup first, for example:
--      pg_dump "$SQL_DSN" > /opt/geili-relay/backup/quota-migration-20260703.sql
--   2. psql -v ON_ERROR_STOP=1 -f scripts/migrate_quota_to_cny100k.sql
--   3. Clear Redis user quota cache after commit.

\set ON_ERROR_STOP on
\if :{?DRY_RUN}
\else
\set DRY_RUN 0
\endif

SELECT CASE WHEN EXISTS (
  SELECT 1
  FROM options
  WHERE key = 'QuotaMigrationVersion'
    AND value = 'cny100k_20260703'
) THEN 1 ELSE 0 END AS quota_migration_already_done
\gset

\if :quota_migration_already_done
UPDATE options
SET value = 'false'
WHERE key = 'QuotaMigrationInProgress';
\echo 'Quota ledger already migrated to cny100k_20260703, skipping.'
\quit 0
\endif

\if :DRY_RUN
\echo 'DRY_RUN=1: executing migration in a transaction that will be rolled back.'
\else
INSERT INTO options (key, value)
VALUES ('QuotaMigrationInProgress', 'true')
ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value;
\endif

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

CREATE TEMP TABLE quota_migration_reconcile (
  table_name text NOT NULL,
  column_name text NOT NULL,
  row_count bigint NOT NULL,
  old_sum numeric NOT NULL,
  expected_sum bigint NOT NULL,
  actual_sum bigint,
  diff_units bigint
) ON COMMIT DROP;

DO $$
DECLARE
  target record;
  row_count bigint;
  old_sum numeric;
  expected_sum bigint;
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
    IF to_regclass(format('public.%I', target.table_name)) IS NOT NULL
      AND EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = 'public'
          AND table_name = target.table_name
          AND column_name = target.column_name
      )
    THEN
      EXECUTE format(
        'SELECT count(*), COALESCE(sum(%1$I::numeric), 0), COALESCE(sum(floor(COALESCE(%1$I, 0)::numeric * 0.2)), 0)::bigint FROM public.%2$I',
        target.column_name,
        target.table_name
      )
      INTO row_count, old_sum, expected_sum;

      INSERT INTO quota_migration_reconcile(table_name, column_name, row_count, old_sum, expected_sum)
      VALUES (target.table_name, target.column_name, row_count, old_sum, expected_sum);
    END IF;
  END LOOP;
END $$;

DO $$
BEGIN
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
  VALUES ('QuotaMigrationVersion', 'cny100k_20260703')
  ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value;

  INSERT INTO options (key, value)
  VALUES ('QuotaMigrationInProgress', 'false')
  ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value;
END $$;

DO $$
DECLARE
  target record;
  new_sum bigint;
BEGIN
  FOR target IN
    SELECT table_name, column_name
    FROM quota_migration_reconcile
  LOOP
    EXECUTE format(
      'SELECT COALESCE(sum(%I::bigint), 0)::bigint FROM public.%I',
      target.column_name,
      target.table_name
    )
    INTO new_sum;

    UPDATE quota_migration_reconcile
    SET actual_sum = new_sum,
        diff_units = new_sum - expected_sum
    WHERE table_name = target.table_name
      AND column_name = target.column_name;
  END LOOP;

  IF EXISTS (
    SELECT 1
    FROM quota_migration_reconcile
    WHERE diff_units <> 0
  ) THEN
    RAISE EXCEPTION 'Quota migration reconciliation failed; inspect quota_migration_reconcile output.';
  END IF;
END $$;

SELECT table_name, column_name, row_count, old_sum, expected_sum, actual_sum, diff_units
FROM quota_migration_reconcile
ORDER BY table_name, column_name;

\if :DRY_RUN
ROLLBACK;
\echo 'DRY_RUN=1: migration transaction rolled back after successful reconciliation.'
\else
COMMIT;
\echo 'Quota migration committed after successful reconciliation.'
\endif
