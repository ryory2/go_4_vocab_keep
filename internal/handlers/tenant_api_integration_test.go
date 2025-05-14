// tenant_api_integration_test.go
package handlers_test

import (
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"go_4_vocab_keep/internal/handlers"
	"go_4_vocab_keep/internal/model"
	"go_4_vocab_keep/internal/repository"
	"go_4_vocab_keep/internal/service"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

var testDB *gorm.DB
var testLogger *slog.Logger

const dbContainerName = "test_postgres_tenant_api"
const dbNetworkName = "docker_my-network"

func TestMain(m *testing.M) {
	testLogger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	slog.SetDefault(testLogger)

	pool, err := dockertest.NewPool("")
	if err != nil {
		log.Fatalf("Could not construct pool: %s", err)
	}
	pool.MaxWait = 120 * time.Second

	var networkExists bool
	networks, err := pool.Client.ListNetworks()
	if err != nil {
		log.Fatalf("Could not list Docker networks: %s", err)
	}
	for _, net := range networks {
		if net.Name == dbNetworkName {
			networkExists = true
			testLogger.Info("Using existing Docker network", slog.String("network_name", dbNetworkName), slog.String("network_id", net.ID))
			break
		}
	}

	if !networkExists {
		_, err = pool.Client.CreateNetwork(docker.CreateNetworkOptions{Name: dbNetworkName})
		if err != nil {
			log.Fatalf("Could not create Docker network %s: %s", dbNetworkName, err)
		}
		testLogger.Info("Docker network created", slog.String("network_name", dbNetworkName))
	}

	resource, err := pool.RunWithOptions(&dockertest.RunOptions{
		Name:       dbContainerName,
		Repository: "postgres",
		Tag:        "15-alpine",
		Env: []string{
			"POSTGRES_USER=user",
			"POSTGRES_PASSWORD=secret",
			"POSTGRES_DB=vocab_keep",
			"listen_addresses = '*'",
		},
		NetworkID: dbNetworkName,
	}, func(config *docker.HostConfig) {
		config.AutoRemove = true
		config.RestartPolicy = docker.RestartPolicy{Name: "no"}
	})
	if err != nil {
		log.Fatalf("Could not start PostgreSQL resource: %s", err)
	}

	hostMappedPort := resource.GetPort("5432/tcp")
	if hostMappedPort == "" {
		if pErr := pool.Purge(resource); pErr != nil {
			log.Printf("Warning: Could not purge resource after failing to get mapped port: %s", pErr)
		}
		log.Fatalf("Could not get mapped port for 5432/tcp from container %s", dbContainerName)
	}

	testLogger.Info("PostgreSQL container started",
		slog.String("container_name", dbContainerName),
		slog.String("container_id_short", resource.Container.ID[:12]),
		slog.String("network_name", dbNetworkName),
		slog.String("host_os_access_via_port", hostMappedPort),
	)

	dbUser := "user"
	dbPassword := "secret"
	dbName := "vocab_keep"
	dbHostForClient := "host.docker.internal"
	dbPortForClient := hostMappedPort

	gormDSN := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable TimeZone=Asia/Tokyo",
		dbHostForClient, dbPortForClient, dbUser, dbPassword, dbName)
	connectionURL := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable&TimeZone=Asia/Tokyo",
		dbUser, dbPassword, dbHostForClient, dbPortForClient, dbName)

	testLogger.Info("Preparing to connect to DB (using host.docker.internal and host-mapped port)",
		slog.String("target_connection_url_format", connectionURL),
		slog.String("gorm_dsn_key_value_format", gormDSN),
		slog.String("note", fmt.Sprintf("Connecting to host '%s' on host-mapped port '%s'. This allows devcontainer to access a service端口 on the host.", dbHostForClient, hostMappedPort)),
	)

	if err = pool.Retry(func() error {
		var errRetry error
		testLogger.Info("Retry: Attempting DB connection...", slog.String("target_url", connectionURL), slog.String("gorm_dsn", gormDSN))
		testDB, errRetry = gorm.Open(postgres.Open(gormDSN), &gorm.Config{
			Logger: gormlogger.Default.LogMode(gormlogger.Silent),
		})
		if errRetry != nil {
			testLogger.Warn("Retry: DB connection attempt failed.", slog.Any("error", errRetry), slog.String("target_url", connectionURL), slog.String("gorm_dsn", gormDSN))
			return errRetry
		}
		sqlDB, errRetry := testDB.DB()
		if errRetry != nil {
			testLogger.Warn("Retry: Failed to get underlying sql.DB from GORM.", slog.Any("error", errRetry))
			if testDB != nil {
			}
			testDB = nil
			return errRetry
		}
		testLogger.Info("Retry: Pinging DB...")
		pingErr := sqlDB.Ping()
		if pingErr != nil {
			testLogger.Warn("Retry: DB ping failed.", slog.Any("error", pingErr))
		} else {
			testLogger.Info("Retry: DB ping successful.")
		}
		return pingErr
	}); err != nil {
		if pErr := pool.Purge(resource); pErr != nil {
			log.Printf("Warning: Could not purge resource after connection retry failed: %s", pErr)
		}
		log.Fatalf("Could not connect to PostgreSQL container after retries: %s. Last URL attempted: %s (GORM DSN: %s)",
			err, connectionURL, gormDSN)
	}
	testLogger.Info("Successfully connected to test PostgreSQL container.", slog.String("connected_using_url_format", connectionURL), slog.String("gorm_dsn_used", gormDSN))

	testLogger.Info("Starting database migration...")
	err = testDB.AutoMigrate(&model.Tenant{}) // ここはTenantモデルに依存
	if err != nil {
		log.Fatalf("Could not migrate database: %s", err)
	}
	testLogger.Info("Database migration completed.")

	testLogger.Info("Running tests...")
	code := m.Run()
	testLogger.Info("Tests finished.", slog.Int("exit_code", code))

	testLogger.Info("Purging PostgreSQL container resource...")
	if err := pool.Purge(resource); err != nil {
		log.Fatalf("Could not purge PostgreSQL resource: %s", err)
	}
	testLogger.Info("PostgreSQL container resource purged.")

	os.Exit(code)
}

type testApp struct {
	router *chi.Mux
	logger *slog.Logger
}

func setupTestApp(t *testing.T) *testApp {
	t.Helper()
	currentLogger := testLogger
	require.NotNil(t, testDB, "TestDB should have been initialized in TestMain")

	tenantRepo := repository.NewGormTenantRepository(currentLogger)
	tenantService := service.NewTenantService(testDB, tenantRepo, currentLogger)
	tenantHandler := handlers.NewTenantHandler(tenantService, currentLogger)

	r := chi.NewRouter()
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(chimiddleware.Recoverer)
	r.Use(chimiddleware.Timeout(5 * time.Second))

	r.Route("/api/v1", func(r chi.Router) {
		r.Post("/tenants", tenantHandler.CreateTenant)
	})
	return &testApp{router: r, logger: currentLogger}
}

// --- レスポンス構造体の定義 (Tenant API固有) ---
type TenantSuccessResponse struct {
	TenantID  uuid.UUID `json:"tenant_id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// APIErrorResponse は helpers_test.go で使用されるため、ここにも残すか、
// プロジェクト共通の型として定義されている場所に移動することを検討。
// ここでは、元のファイルにある想定で残します。
type APIErrorResponse struct {
	Message string `json:"message"`
}

func TestTenantAPI_CreateTenant(t *testing.T) {
	app := setupTestApp(t)
	server := httptest.NewServer(app.router)
	defer server.Close()

	var (
		msgValidationFailed    = "Validation failed"
		msgNameRequired        = "Name"
		msgNameMin             = "min=1"
		msgNameMax             = "max=100"
		msgConflict            = model.ErrConflict.Error()
		msgInvalidInputGeneric = model.ErrInvalidInput.Error()
	)

	testCases := []struct {
		name              string
		payload           interface{}
		setupDB           func(db *gorm.DB)
		expectedCode      int
		expectedErrorMsg  string
		expectedBodyCheck func(t *testing.T, bodyBytes []byte, requestPayload map[string]string) // Tenant固有の検証
		checkDB           func(t *testing.T, db *gorm.DB, requestPayload map[string]string)      // Tenant固有の検証
	}{
		{
			name: "正常系：テナントの作成",
			payload: map[string]string{
				"name": "alpha-tenant-contname-hostport-final",
			},
			setupDB: func(db *gorm.DB) {
				clearTable(t, db, &model.Tenant{}) // ヘルパー関数使用
			},
			expectedCode: http.StatusCreated,
			expectedBodyCheck: func(t *testing.T, bodyBytes []byte, requestPayload map[string]string) {
				var respBody TenantSuccessResponse
				err := json.Unmarshal(bodyBytes, &respBody)
				require.NoError(t, err, "Failed to unmarshal successful response")
				assert.NotEqual(t, uuid.Nil, respBody.TenantID)
				assert.Equal(t, requestPayload["name"], respBody.Name)
				assert.WithinDuration(t, time.Now(), respBody.CreatedAt, 10*time.Second)
				assert.WithinDuration(t, time.Now(), respBody.UpdatedAt, 10*time.Second)
			},
			checkDB: func(t *testing.T, db *gorm.DB, requestPayload map[string]string) {
				var tenantInDB model.Tenant
				err := db.Where("name = ?", requestPayload["name"]).First(&tenantInDB).Error
				require.NoError(t, err, "Tenant should exist in DB")
				assert.Equal(t, requestPayload["name"], tenantInDB.Name)
			},
		},
		{
			name:    "異常系：バリデーションエラー - Nameが指定されていない",
			payload: map[string]string{},
			setupDB: func(db *gorm.DB) {
				clearTable(t, db, &model.Tenant{}) // ヘルパー関数使用
			},
			expectedCode:     http.StatusBadRequest,
			expectedErrorMsg: msgValidationFailed,
		},
		{
			name: "異常系：バリデーションエラー - Nameが空",
			payload: map[string]string{
				"name": "",
			},
			setupDB: func(db *gorm.DB) {
				clearTable(t, db, &model.Tenant{}) // ヘルパー関数使用
			},
			expectedCode:     http.StatusBadRequest,
			expectedErrorMsg: msgValidationFailed,
		},
		{
			name: "異常系：バリデーションエラー - Nameが101文字以上",
			payload: map[string]string{
				"name": strings.Repeat("e", 101),
			},
			setupDB: func(db *gorm.DB) {
				clearTable(t, db, &model.Tenant{}) // ヘルパー関数使用
			},
			expectedCode:     http.StatusBadRequest,
			expectedErrorMsg: msgValidationFailed,
		},
		{
			name: "異常系：処理時エラー - テナント名が重複",
			payload: map[string]string{
				"name": "zeta-duplicate-contname-hostport",
			},
			setupDB: func(db *gorm.DB) {
				clearTable(t, db, &model.Tenant{}) // ヘルパー関数使用
				existingTenant := model.Tenant{TenantID: uuid.New(), Name: "zeta-duplicate-contname-hostport"}
				err := db.Create(&existingTenant).Error
				require.NoError(t, err, "Setup: Failed to create pre-existing tenant")
			},
			expectedCode:     http.StatusConflict,
			expectedErrorMsg: msgConflict,
		},
		{
			name:    "異常系：バリデーションエラー - JSON形式が不正",
			payload: `{"name": "eta-bad-json-contname-hostport", `,
			setupDB: func(db *gorm.DB) {
				clearTable(t, db, &model.Tenant{}) // ヘルパー関数使用
			},
			expectedCode:     http.StatusBadRequest,
			expectedErrorMsg: msgInvalidInputGeneric,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if tc.setupDB != nil {
				tc.setupDB(testDB)
			}

			// ヘルパー関数を使用してリクエストを送信し、基本的なレスポンスを取得
			statusCode, respBodyBytes := sendRequest(t, server,
				httpRequestDetails{
					Method: "POST",
					Path:   "/api/v1/tenants",
					Body:   tc.payload,
				},
				httpResponseExpectations{
					ExpectedCode: tc.expectedCode,
					// ExpectedErrorMsg は sendRequest 内では直接使わず、後続の verifyErrorResponse で使用
				},
			)

			app.logger.Debug("API Response detail",
				slog.String("test_case", tc.name),
				slog.Int("status", statusCode), // sendRequestから返されたステータスコード
				slog.String("body_len", fmt.Sprintf("%d", len(respBodyBytes))),
				slog.String("body_preview", string(respBodyBytes[:minInt(len(respBodyBytes), 200)])), // ヘルパー関数使用
			)

			if statusCode >= 200 && statusCode < 300 { // 成功レスポンス
				if tc.expectedBodyCheck != nil {
					payloadMap := make(map[string]string)
					if p, ok := tc.payload.(map[string]string); ok {
						payloadMap = p
					}
					tc.expectedBodyCheck(t, respBodyBytes, payloadMap)
				}
			} else { // エラーレスポンス
				// ヘルパー関数を使用してエラーレスポンスを検証
				verifyErrorResponse(t, app.logger, respBodyBytes, tc.expectedErrorMsg, tc.name)

				// バリデーションエラーのより詳細なチェック (これはTenant API固有のロジックが残る部分)
				if strings.Contains(tc.name, "Validation Error") && tc.expectedErrorMsg == msgValidationFailed {
					var errResp APIErrorResponse
					if json.Unmarshal(respBodyBytes, &errResp) == nil { // APIErrorResponseとしてパースできる場合のみ
						if strings.Contains(tc.name, "Missing name") {
							assert.True(t, strings.Contains(errResp.Message, msgNameRequired) || strings.Contains(errResp.Message, "required"),
								"Validation err (missing name) should contain '%s' or 'required'. Got: %s", msgNameRequired, errResp.Message)
						} else if strings.Contains(tc.name, "Name empty") || strings.Contains(tc.name, "Name too short") {
							assert.True(t, strings.Contains(errResp.Message, msgNameMin) || strings.Contains(errResp.Message, "min"),
								"Validation err (short name) should contain '%s' or 'min'. Got: %s", msgNameMin, errResp.Message)
						} else if strings.Contains(tc.name, "Name too long") {
							assert.True(t, strings.Contains(errResp.Message, msgNameMax) || strings.Contains(errResp.Message, "max"),
								"Validation err (long name) should contain '%s' or 'max'. Got: %s", msgNameMax, errResp.Message)
						}
					}
				}
			}

			if tc.checkDB != nil {
				payloadMap := make(map[string]string)
				if p, ok := tc.payload.(map[string]string); ok {
					payloadMap = p
				}
				tc.checkDB(t, testDB, payloadMap)
			}
		})
	}
}

// minInt は helpers_test.go に移動したため、ここでは不要
// func min(a, b int) int {
// 	if a < b {
// 		return a
// 	}
// 	return b
// }
