package businessevents

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"net"
	"net/url"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/global-torque/go-common/queue/v3/pclient"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
)

const (
	postgresTestTimeout      = 30 * time.Second
	minimumQueueTrackVersion = 19
)

var (
	postgresTestSettings     = []string{"DB_TYPE", "DB_HOST", "DB_PORT", "DB_USER", "DB_PASSWORD", "DB_DATABASE", "DB_APP_NAME", "DB_SSL_MODE"}
	postgresRequiredSettings = []string{"DB_TYPE", "DB_USER", "DB_PASSWORD", "DB_DATABASE", "DB_APP_NAME"}
)

func TestPostgresAtomicReplayAndConflict(t *testing.T) {
	config := postgresTestConfig(t)

	ctx, cancel := postgresTestContext()
	defer cancel()

	conn, err := pgx.ConnectConfig(ctx, config)
	if err != nil {
		t.Fatalf("connect to configured PostgreSQL database: %s", redactSecrets(err.Error(), config.User, config.Password))
	}
	t.Cleanup(func() {
		closeCtx, closeCancel := postgresTestContext()
		defer closeCancel()
		if closeErr := conn.Close(closeCtx); closeErr != nil {
			t.Errorf("close PostgreSQL test connection: %v", closeErr)
		}
	})

	requireCanonicalPostgresFixture(t, ctx, conn)

	_, err = conn.Exec(ctx, "CREATE TEMP TABLE business_event_test_state (id uuid PRIMARY KEY)")
	require.NoError(t, err)

	event := sample(t)
	event.ID = uuid.NewString()
	originalPayload, err := Marshal(event)
	require.NoError(t, err)

	tx, err := conn.Begin(ctx)
	require.NoError(t, err)
	t.Cleanup(func() { rollbackTestTransaction(t, tx) })

	_, err = tx.Exec(ctx, "INSERT INTO pg_temp.business_event_test_state (id) VALUES ($1::uuid)", event.ID)
	require.NoError(t, err)

	inserted, err := Insert(ctx, tx, event)
	require.NoError(t, err)
	require.True(t, inserted)

	inserted, err = Insert(ctx, tx, event)
	require.NoError(t, err)
	require.False(t, inserted)

	conflictingEvent := event
	conflictingEvent.Data = maps.Clone(event.Data)
	conflictingEvent.Data["changed"] = true
	_, err = Insert(ctx, tx, conflictingEvent)
	require.ErrorIs(t, err, ErrConflict)

	// ErrConflict is an application result, not an aborted PostgreSQL transaction.
	requireStateCount(t, ctx, tx, event.ID, 1)
	requireStoredEvent(t, ctx, tx, event, originalPayload)
	unchangedPayload, err := Marshal(event)
	require.NoError(t, err)
	require.JSONEq(t, string(originalPayload), string(unchangedPayload))
	require.NotContains(t, event.Data, "changed")

	require.NoError(t, tx.Rollback(ctx))
	requireEventCount(t, ctx, conn, event.ID, 0)
	requireStateCount(t, ctx, conn, event.ID, 0)
}

func TestPostgresTestConfigUnavailable(t *testing.T) {
	config, unavailable, err := parsePostgresTestConfig(func(string) string { return "" })
	require.NoError(t, err)
	require.True(t, unavailable)
	require.Nil(t, config)
}

func TestPostgresTestConfigRejectsPartialEnvironment(t *testing.T) {
	values := map[string]string{"DB_TYPE": "postgres"}
	_, unavailable, err := parsePostgresTestConfig(func(name string) string { return values[name] })
	require.False(t, unavailable)
	require.EqualError(t, err, "partial PostgreSQL test configuration; missing DB_USER, DB_PASSWORD, DB_DATABASE, DB_APP_NAME")
}

func TestPostgresTestConfigRecognizesEveryConnectionSetting(t *testing.T) {
	settings := []struct {
		name  string
		value string
	}{
		{name: "DB_TYPE", value: "postgres"},
		{name: "DB_HOST", value: "localhost"},
		{name: "DB_PORT", value: "not-a-port"},
		{name: "DB_USER", value: "fixture_user"},
		{name: "DB_PASSWORD", value: "fixture_password"},
		{name: "DB_DATABASE", value: "invest_platform"},
		{name: "DB_APP_NAME", value: "business_event_test"},
		{name: "DB_SSL_MODE", value: "disable"},
	}
	for _, setting := range settings {
		t.Run(setting.name, func(t *testing.T) {
			values := map[string]string{setting.name: setting.value}

			config, unavailable, err := parsePostgresTestConfig(func(key string) string { return values[key] })

			require.Error(t, err)
			require.False(t, unavailable)
			require.Nil(t, config)
			if setting.name == "DB_HOST" || setting.name == "DB_PORT" || setting.name == "DB_SSL_MODE" {
				require.EqualError(t, err,
					"partial PostgreSQL test configuration; missing DB_TYPE, DB_USER, DB_PASSWORD, DB_DATABASE, DB_APP_NAME")
			}
		})
	}
}

func TestPostgresTestConfigDefaults(t *testing.T) {
	values := completePostgresTestSettings()
	config, unavailable, err := parsePostgresTestConfig(func(name string) string { return values[name] })
	require.NoError(t, err)
	require.False(t, unavailable)
	require.Equal(t, "localhost", config.Host)
	require.Equal(t, uint16(5432), config.Port)
	require.Nil(t, config.TLSConfig)
	require.Equal(t, values["DB_USER"], config.User)
	require.Equal(t, values["DB_PASSWORD"], config.Password)
	require.Equal(t, values["DB_DATABASE"], config.Database)
	require.Equal(t, values["DB_APP_NAME"], config.RuntimeParams["application_name"])
}

func TestPostgresTestConfigRejectsInvalidValuesWithoutLeakingPassword(t *testing.T) {
	values := completePostgresTestSettings()
	values["DB_PORT"] = "not-a-port"
	_, unavailable, err := parsePostgresTestConfig(func(name string) string { return values[name] })
	require.False(t, unavailable)
	require.Error(t, err)
	require.ErrorContains(t, err, "not-a-port")
	require.NotContains(t, err.Error(), values["DB_PASSWORD"])

	values = completePostgresTestSettings()
	values["DB_TYPE"] = "mysql"
	_, unavailable, err = parsePostgresTestConfig(func(name string) string { return values[name] })
	require.False(t, unavailable)
	require.EqualError(t, err, `DB_TYPE must be postgres, got "mysql"`)
}

func completePostgresTestSettings() map[string]string {
	return map[string]string{
		"DB_TYPE":     "postgres",
		"DB_USER":     "fixture_user",
		"DB_PASSWORD": "do-not-leak-this-password",
		"DB_DATABASE": "invest_platform",
		"DB_APP_NAME": "business_event_test",
	}
}

func postgresTestConfig(t *testing.T) *pgx.ConnConfig {
	t.Helper()

	config, unavailable, err := parsePostgresTestConfig(lookupPostgresTestEnvironment)
	if unavailable {
		t.Skip("external PostgreSQL fixture unavailable: all recognized DB_* settings are absent")
	}
	if err != nil {
		t.Fatal(err)
	}

	return config
}

func parsePostgresTestConfig(getenv func(string) string) (*pgx.ConnConfig, bool, error) {
	configured := false
	for _, name := range postgresTestSettings {
		if getenv(name) != "" {
			configured = true
			break
		}
	}
	if !configured {
		return nil, true, nil
	}

	missing := make([]string, 0, len(postgresRequiredSettings))
	for _, name := range postgresRequiredSettings {
		if getenv(name) == "" {
			missing = append(missing, name)
		}
	}

	if len(missing) != 0 {
		return nil, false, fmt.Errorf("partial PostgreSQL test configuration; missing %s", strings.Join(missing, ", "))
	}

	databaseType := getenv("DB_TYPE")
	if databaseType != "postgres" {
		return nil, false, fmt.Errorf("DB_TYPE must be postgres, got %q", databaseType)
	}

	host := getenv("DB_HOST")
	if host == "" {
		host = "localhost"
	}
	port := getenv("DB_PORT")
	if port == "" {
		port = "5432"
	}
	sslMode := getenv("DB_SSL_MODE")
	if sslMode == "" {
		sslMode = "disable"
	}

	// Parse a credential-free URL so parser errors can retain their underlying
	// detail without disclosing DB_USER or DB_PASSWORD.
	dsn := (&url.URL{
		Scheme: databaseType,
		Host:   net.JoinHostPort(host, port),
		Path:   getenv("DB_DATABASE"),
		RawQuery: url.Values{
			"application_name": []string{getenv("DB_APP_NAME")},
			"sslmode":          []string{sslMode},
		}.Encode(),
	}).String()

	config, err := pgx.ParseConfig(dsn)
	if err != nil {
		return nil, false, fmt.Errorf("parse PostgreSQL test configuration: %w", err)
	}
	config.User = getenv("DB_USER")
	config.Password = getenv("DB_PASSWORD")
	config.Database = getenv("DB_DATABASE")

	return config, false, nil
}

func lookupPostgresTestEnvironment(name string) string {
	switch name {
	case "DB_TYPE", "DB_HOST", "DB_PORT", "DB_USER", "DB_PASSWORD", "DB_DATABASE", "DB_APP_NAME", "DB_SSL_MODE":
		return os.Getenv(name)
	default:
		return ""
	}
}

func requireCanonicalPostgresFixture(t *testing.T, ctx context.Context, conn *pgx.Conn) {
	t.Helper()

	names := []string{"migration_services", "migration_service_logs", "business_events"}
	present := make([]bool, len(names))
	err := conn.QueryRow(ctx, `SELECT
		to_regclass('public.migration_services') IS NOT NULL,
		to_regclass('public.migration_service_logs') IS NOT NULL,
		to_regclass('public.business_events') IS NOT NULL`).Scan(&present[0], &present[1], &present[2])
	require.NoError(t, err)

	presentCount := 0
	for _, exists := range present {
		if exists {
			presentCount++
		}
	}

	if presentCount != len(names) {
		states := make([]string, 0, len(names))
		for i, name := range names {
			state := "absent"
			if present[i] {
				state = "present"
			}
			states = append(states, name+"="+state)
		}
		t.Fatalf("inconsistent external PostgreSQL fixture: %s", strings.Join(states, ", "))
	}

	var queueVersion int
	err = conn.QueryRow(ctx, "SELECT version FROM public.migration_services WHERE name = 'queue'").Scan(&queueVersion)
	if errors.Is(err, pgx.ErrNoRows) {
		t.Fatal("canonical PostgreSQL fixture has no queue migration track")
	}
	require.NoError(t, err)
	require.GreaterOrEqual(t, queueVersion, minimumQueueTrackVersion, "queue migration 19 must be applied before consumer enablement")

	requireBusinessEventsColumns(t, ctx, conn)
	requireBusinessEventsPrimaryKey(t, ctx, conn)
	requireBusinessEventsHistoryIndex(t, ctx, conn)
	requireBusinessEventsTriggers(t, ctx, conn)
}

type postgresColumnContract struct {
	typeName string
	notNull  bool
	default_ string
}

func requireBusinessEventsColumns(t *testing.T, ctx context.Context, conn *pgx.Conn) {
	t.Helper()

	rows, err := conn.Query(ctx, `SELECT
		a.attname,
		format_type(a.atttypid, a.atttypmod),
		a.attnotnull,
		COALESCE(pg_get_expr(d.adbin, d.adrelid), '')
	FROM pg_catalog.pg_attribute AS a
	JOIN pg_catalog.pg_class AS c ON c.oid = a.attrelid
	JOIN pg_catalog.pg_namespace AS n ON n.oid = c.relnamespace
	LEFT JOIN pg_catalog.pg_attrdef AS d ON d.adrelid = a.attrelid AND d.adnum = a.attnum
	WHERE n.nspname = 'public' AND c.relname = 'business_events'
		AND a.attnum > 0 AND NOT a.attisdropped`)
	require.NoError(t, err)
	defer rows.Close()

	actual := make(map[string]postgresColumnContract)
	for rows.Next() {
		var name string
		var column postgresColumnContract
		require.NoError(t, rows.Scan(&name, &column.typeName, &column.notNull, &column.default_))
		actual[name] = column
	}
	require.NoError(t, rows.Err())

	expected := map[string]postgresColumnContract{
		"id":            {typeName: "uuid", notNull: true},
		"aggregatetype": {typeName: "text", notNull: true},
		"aggregateid":   {typeName: "text", notNull: true},
		"type":          {typeName: "text", notNull: true},
		"payload":       {typeName: "jsonb", notNull: true},
		"occurred_at":   {typeName: "timestamp with time zone", notNull: true},
		"created_at":    {typeName: "timestamp with time zone", notNull: true, default_: "now()"},
		"updated_at":    {typeName: "timestamp with time zone", notNull: true, default_: "now()"},
	}
	for name, contract := range expected {
		column, exists := actual[name]
		require.True(t, exists, "public.business_events is missing required column %s", name)
		require.Equal(t, contract, column, "public.business_events column %s differs from migration 01_queue/19_business_events.sql", name)
	}
}

func requireBusinessEventsPrimaryKey(t *testing.T, ctx context.Context, conn *pgx.Conn) {
	t.Helper()

	var columns []string
	err := conn.QueryRow(ctx, `SELECT ARRAY(
		SELECT a.attname
		FROM unnest(con.conkey) WITH ORDINALITY AS key(attnum, position)
		JOIN pg_catalog.pg_attribute AS a ON a.attrelid = con.conrelid AND a.attnum = key.attnum
		ORDER BY key.position
	)
	FROM pg_catalog.pg_constraint AS con
	WHERE con.conrelid = 'public.business_events'::regclass AND con.contype = 'p'`).Scan(&columns)
	require.NoError(t, err, "public.business_events must have a primary key")
	require.Equal(t, []string{"id"}, columns, "public.business_events primary key must contain only id")
}

func requireBusinessEventsHistoryIndex(t *testing.T, ctx context.Context, conn *pgx.Conn) {
	t.Helper()

	var (
		valid, ready, unpredicated, expressionFree, canonicalOrder bool
		attributeCount, keyAttributeCount                          int
		columns                                                    []string
	)
	err := conn.QueryRow(ctx, `SELECT
		i.indisvalid,
		i.indisready,
		i.indpred IS NULL,
		i.indexprs IS NULL,
		NOT EXISTS (SELECT 1 FROM unnest(i.indoption::smallint[]) AS option(value) WHERE option.value <> 0),
		i.indnatts,
		i.indnkeyatts,
		ARRAY(
			SELECT a.attname
			FROM unnest(i.indkey::smallint[]) WITH ORDINALITY AS key(attnum, position)
			JOIN pg_catalog.pg_attribute AS a ON a.attrelid = i.indrelid AND a.attnum = key.attnum
			ORDER BY key.position
		)
	FROM pg_catalog.pg_index AS i
	JOIN pg_catalog.pg_class AS index_class ON index_class.oid = i.indexrelid
	JOIN pg_catalog.pg_am AS access_method ON access_method.oid = index_class.relam
	JOIN pg_catalog.pg_namespace AS index_namespace ON index_namespace.oid = index_class.relnamespace
	WHERE i.indrelid = 'public.business_events'::regclass
		AND index_namespace.nspname = 'public'
		AND index_class.relname = 'business_events_aggregate_history_idx'
		AND access_method.amname = 'btree'`).Scan(
		&valid, &ready, &unpredicated, &expressionFree, &canonicalOrder,
		&attributeCount, &keyAttributeCount, &columns,
	)
	require.NoError(t, err, "public.business_events is missing business_events_aggregate_history_idx")
	require.True(t, valid, "business_events_aggregate_history_idx must be valid")
	require.True(t, ready, "business_events_aggregate_history_idx must be ready")
	require.True(t, unpredicated, "business_events_aggregate_history_idx must not be partial")
	require.True(t, expressionFree, "business_events_aggregate_history_idx must contain only columns")
	require.True(t, canonicalOrder, "business_events_aggregate_history_idx must use canonical ascending order")
	require.Equal(t, 4, attributeCount, "business_events_aggregate_history_idx must not contain included columns")
	require.Equal(t, 4, keyAttributeCount)
	require.Equal(t, []string{"aggregatetype", "aggregateid", "occurred_at", "id"}, columns)
}

type postgresTriggerContract struct {
	functionSchema string
	functionName   string
	typeMask       int16
}

func requireBusinessEventsTriggers(t *testing.T, ctx context.Context, conn *pgx.Conn) {
	t.Helper()

	expected := map[string]postgresTriggerContract{
		"business_events_immutable": {
			functionSchema: "public",
			functionName:   "reject_business_event_mutation",
			typeMask:       27, // BEFORE UPDATE OR DELETE, EACH ROW.
		},
		"business_events_no_truncate": {
			functionSchema: "public",
			functionName:   "reject_business_event_mutation",
			typeMask:       34, // BEFORE TRUNCATE, EACH STATEMENT.
		},
		"business_events_set_timestamp": {
			functionSchema: "public",
			functionName:   "trigger_set_timestamp",
			typeMask:       19, // BEFORE UPDATE, EACH ROW.
		},
	}

	rows, err := conn.Query(ctx, `SELECT
		t.tgname,
		t.tgenabled,
		t.tgtype,
		function_namespace.nspname,
		function.proname
	FROM pg_catalog.pg_trigger AS t
	JOIN pg_catalog.pg_proc AS function ON function.oid = t.tgfoid
	JOIN pg_catalog.pg_namespace AS function_namespace ON function_namespace.oid = function.pronamespace
	WHERE t.tgrelid = 'public.business_events'::regclass
		AND NOT t.tgisinternal
		AND t.tgname = ANY($1::text[])`, slices.Collect(maps.Keys(expected)))
	require.NoError(t, err)
	defer rows.Close()

	found := make(map[string]bool, len(expected))
	for rows.Next() {
		var name, enabled, functionSchema, functionName string
		var typeMask int16
		require.NoError(t, rows.Scan(&name, &enabled, &typeMask, &functionSchema, &functionName))

		contract := expected[name]
		require.Contains(t, []string{"O", "A"}, enabled, "%s must be enabled for ordinary writes", name)
		require.Equal(t, contract.typeMask, typeMask, "%s must retain its exact BEFORE event and row/statement timing", name)
		require.Equal(t, contract.functionSchema, functionSchema, "%s function schema differs", name)
		require.Equal(t, contract.functionName, functionName, "%s function binding differs", name)
		found[name] = true
	}
	require.NoError(t, rows.Err())
	for name := range expected {
		require.True(t, found[name], "public.business_events is missing enabled trigger %s", name)
	}
}

func postgresTestContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), postgresTestTimeout)
}

func rollbackTestTransaction(t *testing.T, tx pgx.Tx) {
	t.Helper()

	ctx, cancel := postgresTestContext()
	defer cancel()
	err := tx.Rollback(ctx)
	if err != nil && !errors.Is(err, pgx.ErrTxClosed) {
		t.Errorf("roll back test transaction: %v", err)
	}
}

func requireEventCount(t *testing.T, ctx context.Context, queryer QueryRowExecutor, eventID string, expected int) {
	t.Helper()

	var count int
	require.NoError(t, queryer.QueryRow(ctx, "SELECT count(*) FROM public.business_events WHERE id=$1::uuid", eventID).Scan(&count))
	require.Equal(t, expected, count)
}

func requireStateCount(t *testing.T, ctx context.Context, queryer QueryRowExecutor, stateID string, expected int) {
	t.Helper()

	var count int
	require.NoError(t, queryer.QueryRow(ctx, "SELECT count(*) FROM pg_temp.business_event_test_state WHERE id=$1::uuid", stateID).Scan(&count))
	require.Equal(t, expected, count)
}

func requireStoredEvent(
	t *testing.T,
	ctx context.Context,
	queryer QueryRowExecutor,
	expected pclient.Event,
	expectedPayload []byte,
) {
	t.Helper()

	var id, aggregateType, aggregateID, eventType, payload string
	var occurredAt, createdAt, updatedAt time.Time
	require.NoError(t, queryer.QueryRow(ctx, `SELECT id::text, aggregatetype, aggregateid, type, payload::text, occurred_at, created_at, updated_at
		FROM public.business_events WHERE id=$1::uuid`, expected.ID).Scan(
		&id, &aggregateType, &aggregateID, &eventType, &payload, &occurredAt, &createdAt, &updatedAt,
	))
	require.Equal(t, expected.ID, id)
	require.Equal(t, expected.ObjectName, aggregateType)
	require.Equal(t, expected.ObjectName+":"+expected.ObjectRef, aggregateID)
	require.Equal(t, string(expected.Action), eventType)
	require.JSONEq(t, string(expectedPayload), payload)
	require.True(t, expected.OccurredAt.Equal(occurredAt), "stored occurred_at %s differs from event occurred_at %s", occurredAt, expected.OccurredAt)
	require.False(t, createdAt.IsZero())
	require.True(t, createdAt.Equal(updatedAt), "new row timestamps differ: created_at=%s updated_at=%s", createdAt, updatedAt)
}

func redactSecrets(message string, secrets ...string) string {
	for _, secret := range secrets {
		if secret != "" {
			message = strings.ReplaceAll(message, secret, "[redacted]")
		}
	}

	return message
}
