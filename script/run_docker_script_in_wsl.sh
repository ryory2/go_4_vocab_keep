#!/bin/bash

# スクリプトをエラーがあれば停止する設定
set -e

#############################################
# パラメータ設定
#############################################

# ルートディレクトリを取得
ROOT_DIR=$1

# 環境名を取得
ENV_NAME=$2

# dockerディレクトリを取得
DOCKER_DIR="$ROOT_DIR/docker"

# ログディレクトリのパスを設定（相対パスまたは絶対パス）
LOG_DIR="$ROOT_DIR/script/logs"

# ログファイル名を設定
LOG_FILE="docker_execution_log_in_linux.$ENV_NAME.log"

# 環境変数ファイルのパスを設定
ENV_FILE="$DOCKER_DIR/env/.env.$ENV_NAME"

# docker-composeのパスを設定
COMPOSE_FILE="$DOCKER_DIR/docker-compose.$ENV_NAME.yml"

# Docker Composeコマンドのオプションを設定
DOCKER_COMPOSE_OPTS="--file ${COMPOSE_FILE} --env-file ${ENV_FILE}"

#############################################
# ログディレクトリとログファイルの準備
#############################################

# dockerフォルダの存在を確認し、存在しない場合は作成
mkdir -p "$LOG_DIR"

# 完全なログファイルパスを設定
FULL_LOG_FILE="${LOG_DIR}/${LOG_FILE}"

# ログファイルが存在する場合は削除
if [ -f "${FULL_LOG_FILE}" ]; then
    rm "${FULL_LOG_FILE}"
fi

#############################################
# ログ記録の開始
#############################################

echo "スクリプトが実行されました。" >> "${FULL_LOG_FILE}"
echo "引数1: $ROOT_DIR" >> "${FULL_LOG_FILE}"
echo "引数1: $ENV_NAME" >> "${FULL_LOG_FILE}"
echo "引数1: $DOCKER_DIR" >> "${FULL_LOG_FILE}"
echo "引数1: $LOG_DIR" >> "${FULL_LOG_FILE}"
echo "引数1: $LOG_FILE" >> "${FULL_LOG_FILE}"
echo "引数1: $ENV_FILE" >> "${FULL_LOG_FILE}"
echo "引数1: $COMPOSE_FILE" >> "${FULL_LOG_FILE}"
echo "引数1: $DOCKER_COMPOSE_OPTS" >> "${FULL_LOG_FILE}"

# 実行開始時刻をログに記録
{
    echo "----------------------------------------------------------------------------------------------------"
    echo "Execution started at $(date)"
} >> "${FULL_LOG_FILE}"

# Docker開始のログ
echo "-------------------------docker command start-------------------------" >> "${FULL_LOG_FILE}"

#############################################
# Dockerコマンドの実行
#############################################

# ドッカーバージョンを確認（必要に応じてコメント解除）
# echo "■Running docker -v..." | tee -a "${FULL_LOG_FILE}"
# docker -v >> "${FULL_LOG_FILE}" 2>&1

# Docker Composeの設定を確認
echo "■Running docker compose config..." | tee -a "${FULL_LOG_FILE}"
echo "docker compose ${DOCKER_COMPOSE_OPTS} config" >> "${FULL_LOG_FILE}"
docker compose ${DOCKER_COMPOSE_OPTS} config >> "${FULL_LOG_FILE}" 2>&1

# Docker Composeを停止（必要に応じてコメント解除）
# echo "■Running docker compose down..." | tee -a "${FULL_LOG_FILE}"
# docker compose ${DOCKER_COMPOSE_OPTS} down >> "${FULL_LOG_FILE}" 2>&1

# Docker Composeのビルド
echo "■Running docker compose build..." | tee -a "${FULL_LOG_FILE}"
echo "docker compose ${DOCKER_COMPOSE_OPTS} build --no-cache" >> "${FULL_LOG_FILE}"
docker compose ${DOCKER_COMPOSE_OPTS} build --no-cache >> "${FULL_LOG_FILE}" 2>&1

# Docker Composeの起動
echo "■Running docker compose up..." | tee -a "${FULL_LOG_FILE}"
echo "docker compose ${DOCKER_COMPOSE_OPTS} up -d" >> "${FULL_LOG_FILE}"
docker compose ${DOCKER_COMPOSE_OPTS} up -d >> "${FULL_LOG_FILE}" 2>&1

# Docker完了のログ
echo "-------------------------docker command end-------------------------" >> "${FULL_LOG_FILE}"

#############################################
# ログ記録の終了
#############################################

# 実行完了時刻をログに記録
{
    echo "Execution finished at $(date)"
    echo "----------------------------------------------------------------------------------------------------"
} >> "${FULL_LOG_FILE}"

# 実行結果を表示
echo "Docker control script executed. Logs are available at ${FULL_LOG_FILE}"
