// internal/config/constants.go
package config

// アプリケーション情報
const (
	AppName    = "MyAwesomeApp"
	AppVersion = "1.0.2"
)

// デフォルト設定値
const (
	DefaultServerPort     = ":8080"
	DefaultLogLevel       = "info"
	DefaultLogFormat      = "json"
	DefaultAppReviewLimit = 20
	DefaultAuthEnabled    = false
)

// 特定の外部サービスのエンドポイントなど
const PaymentGatewayAPIEndpoint = "https://api.paymentprovider.com/v1"
