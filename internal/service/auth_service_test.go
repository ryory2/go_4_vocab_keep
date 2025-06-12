package service_test // メインコードとは別のパッケージにすることで、公開されているものしかテストできなくなり、より良いテストになる

import (
	"context"
	"testing"
	"time"

	"go_4_vocab_keep/internal/config"
	"go_4_vocab_keep/internal/model"
	"go_4_vocab_keep/internal/repository/mocks"
	"go_4_vocab_keep/internal/service"
	servicemocks "go_4_vocab_keep/internal/service/mocks" // Mailerのモック

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

// --- テストスイートの定義 ---
// 関連するテストと、共通のセットアップをまとめる
type AuthServiceTestSuite struct {
	suite.Suite // testifyのSuiteを埋め込む

	mockTenantRepo *mocks.TenantRepository
	mockTokenRepo  *mocks.TokenRepository
	mockMailer     *servicemocks.Mailer
	cfg            *config.Config
	authService    service.AuthService
}

// --- セットアップメソッド ---
// 各テスト(`TestXxx`)が実行される直前に呼ばれる
func (s *AuthServiceTestSuite) SetupTest() {
	// 各テストの前に、モックを新しく生成してクリーンな状態にする
	s.mockTenantRepo = new(mocks.TenantRepository)
	s.mockTokenRepo = new(mocks.TokenRepository)
	s.mockMailer = new(servicemocks.Mailer)

	// テスト用のダミー設定
	s.cfg = &config.Config{
		App: config.AppConfig{FrontendURL: "http://localhost:3000"},
		JWT: config.JWTConfig{
			SecretKey:      "test-secret",
			AccessTokenTTL: 15 * time.Minute,
		},
	}

	// テスト対象のサービスにモックを注入してインスタンスを生成
	s.authService = service.NewAuthService(nil, s.mockTenantRepo, s.mockTokenRepo, s.mockMailer, s.cfg)
}

// --- テストランナー ---
// この関数が `go test` から実際に呼ばれる
func TestAuthService(t *testing.T) {
	suite.Run(t, new(AuthServiceTestSuite))
}

// --- RegisterTenantメソッドのテスト ---
func (s *AuthServiceTestSuite) TestRegisterTenant() {
	// テストケースをテーブルとして定義
	testCases := []struct {
		name        string // テストケース名
		req         *model.RegisterRequest
		setupMocks  func()                                // このケースのためのモック設定
		checkResult func(tenant *model.Tenant, err error) // 結果の検証
	}{
		{
			name: "Success - 正常に登録できる",
			req:  &model.RegisterRequest{Name: "test", Email: "test@example.com", Password: "password"},
			setupMocks: func() {
				// 正常系のモック設定
				s.mockTenantRepo.On("FindByEmail", mock.Anything, mock.Anything, "test@example.com").Return(nil, model.ErrNotFound).Once()
				s.mockTenantRepo.On("FindByName", mock.Anything, mock.Anything, "test").Return(nil, model.ErrNotFound).Once()
				s.mockTenantRepo.On("Create", mock.Anything, mock.Anything, mock.AnythingOfType("*model.Tenant")).Return(nil).Once()
				s.mockTokenRepo.On("CreateVerificationToken", mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
				s.mockMailer.On("Send", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
			},
			checkResult: func(tenant *model.Tenant, err error) {
				s.NoError(err)
				s.NotNil(tenant)
				s.Equal("test@example.com", tenant.Email)
			},
		},
		{
			name: "Failure - Emailが重複している",
			req:  &model.RegisterRequest{Name: "test", Email: "test@example.com", Password: "password"},
			setupMocks: func() {
				// Email重複時のモック設定
				s.mockTenantRepo.On("FindByEmail", mock.Anything, mock.Anything, "test@example.com").Return(&model.Tenant{}, nil).Once()
			},
			checkResult: func(tenant *model.Tenant, err error) {
				s.Nil(tenant)
				s.Error(err)
				var appErr *model.AppError
				s.ErrorAs(err, &appErr)
				s.Equal("DUPLICATE_EMAIL", appErr.Detail.Code)
			},
		},
		// ここに他のテストケース（名前の重複、DBエラーなど）を追加していく
	}

	// テーブルのループ実行
	for _, tc := range testCases {
		s.Run(tc.name, func() {
			// SetupTestが呼ばれてモックがリセットされる
			s.SetupTest()

			// 1. Arrange (準備)
			tc.setupMocks()

			// 2. Act (実行)
			createdTenant, err := s.authService.RegisterTenant(context.Background(), tc.req)

			// 3. Assert (検証)
			tc.checkResult(createdTenant, err)

			// モックの呼び出しが期待通りだったか全体を検証
			s.mockTenantRepo.AssertExpectations(s.T())
			s.mockTokenRepo.AssertExpectations(s.T())
			s.mockMailer.AssertExpectations(s.T())
		})
	}
}

// --- Loginメソッドのテストも同様に記述 ---
func (s *AuthServiceTestSuite) TestLogin() {
	// ... Login用のテストケーステーブルを定義 ...
}
