## 概要
- エンドポイント /、/healthだけのgoのアプリケーション
- terraformの勉強用で作成

## 起動方法
- docker/go/Dockerfile.localを用意
- 以下を実行
    script/run_docker_compose.local.bat

## ECRへのプッシュ
- arn
    990606419933.dkr.ecr.ap-northeast-1.amazonaws.com/test/go_4_vocab_keep

## フォルダの作成
mkdir -p cmd internal pkg api web configs scripts build test
mkdir -p internal/handlers \
         internal/models \
         internal/store \
         internal/service \
         internal/config \
         internal/middleware
# --- web のサブディレクトリを作成 ---
mkdir -p web/static \
         web/template
# --- build のサブディレクトリを作成 ---
mkdir -p build/package
# --- test のサブディレクトリを作成 ---
mkdir -p test/e2e

## ディレクトリ構成図
myproject/
├── go.mod                      # Go Modules: 依存関係とモジュールパスを定義
├── go.sum                      # Go Modules: 依存関係のチェックサム
├── README.md                   # プロジェクトの説明
├── .gitignore                  # Git で無視するファイル/ディレクトリのリスト
│
├── cmd/                        # 実行可能ファイルのエントリーポイント (main パッケージ)
│   └── myapp/                  # アプリケーション名 (例: myapp)
│       └── main.go             # myapp のエントリーポイント
│   └── anotherapp/             # 別の実行可能ファイル (もしあれば)
│       └── main.go
│
├── internal/                   # このプロジェクト内部でのみインポート可能なコード
│   ├── handlers/               # HTTP ハンドラ、gRPC サーバー実装など (内部実装)
│   │   └── user_handler.go     # (例)
│   │   └── product_handler.go
│   ├── service/                 # ビジネスロジック (ユースケース層)
│   │   ├── user_service.go
│   │   └── product_service.go
│   ├── repository/             # データ永続化層 (DBアクセスなど)
│   │   ├── user_repository.go
│   │   └── product_repository.go
│   ├── model/                  # データ構造の定義 (ドメインモデル、DTOなど)
│   │   ├── user.go
│   │   └── product.go
│   ├── config/                 # 設定の読み込み・管理
│   │   └── config.go
│   └── middleware/             # HTTPミドルウェア (認証、ロギングなど)
│       └── auth.go
│
├── pkg/                        # 外部プロジェクトからのインポートを許可するライブラリコード (※議論あり)
│   └── logger/                 # 公開可能なロギングライブラリ (例)
│   │   └── logger.go
│   └── errors/                 # 公開可能なカスタムエラー (例)
│   │    └── errors.go
│   └── validator/               # 例: カスタムバリデータ
│       └── validator.go
│
├── api/                        # APIスキーマ定義 (OpenAPI/Swagger, Protocol Buffersなど) (オプション)
│   └── swagger.yaml
│
├── configs/                    # 設定ファイル
│   └── config.yaml.example     # 設定ファイルのサンプル
│
├── scripts/                    # ビルド、デプロイ、分析などのヘルパースクリプト
│   └── build.sh                # (例)
│
├── test/                       # 追加のテスト (E2E, インテグレーションテストなど)
│   └── e2e/                    # エンドツーエンドテスト (例)
│       └── main_test.go


# ビジネスロジックテスト
go test -v ./internal/service/

# ハンドラーてえ嘘t
go test -v ./internal/service/




# ブランチ運用
本番用ブランチ: main
開発用ブランチ: develop