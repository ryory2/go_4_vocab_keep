#!/bin/bash

# スクリプトが失敗した場合に即座に終了
set -e

# ベースディレクトリ
BASE_DIR="src"

# ディレクトリの作成
echo "ディレクトリを作成しています..."

mkdir -p "$BASE_DIR/api"
mkdir -p "$BASE_DIR/components/common"
mkdir -p "$BASE_DIR/components/ErrorPage"
mkdir -p "$BASE_DIR/components/TaskDetail"
mkdir -p "$BASE_DIR/components/TaskForm"
mkdir -p "$BASE_DIR/components/TaskList"
mkdir -p "$BASE_DIR/pages"
mkdir -p "$BASE_DIR/routes"
mkdir -p "$BASE_DIR/types"

echo "ディレクトリの作成が完了しました。"

# ファイルの作成
echo "ファイルを作成しています..."

# APIフォルダ内
touch "$BASE_DIR/api/apiClient.ts"

# 共通コンポーネント
touch "$BASE_DIR/components/common/Header.tsx"
touch "$BASE_DIR/components/common/Footer.tsx"

# エラーページコンポーネント
touch "$BASE_DIR/components/ErrorPage/ErrorPage.tsx"

# タスク詳細コンポーネント
touch "$BASE_DIR/components/TaskDetail/TaskDetail.tsx"

# タスクフォームコンポーネント
touch "$BASE_DIR/components/TaskForm/TaskForm.tsx"

# タスクリストコンポーネント
touch "$BASE_DIR/components/TaskList/TaskItem.tsx"
touch "$BASE_DIR/components/TaskList/TaskList.tsx"

# ページコンポーネント
touch "$BASE_DIR/pages/ErrorPage.tsx"
touch "$BASE_DIR/pages/TaskCreatePage.tsx"
touch "$BASE_DIR/pages/TaskDetailPage.tsx"
touch "$BASE_DIR/pages/TaskEditPage.tsx"
touch "$BASE_DIR/pages/TaskListPage.tsx"

# ルーティング設定
touch "$BASE_DIR/routes/AppRouter.tsx"

# 型定義
touch "$BASE_DIR/types/Task.ts"

# アプリケーションのエントリーポイント
touch "$BASE_DIR/App.tsx"
touch "$BASE_DIR/index.tsx"

echo "ファイルの作成が完了しました。"
echo "フォルダ構成とファイルが正常に作成されました。"
