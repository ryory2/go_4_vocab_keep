#!/bin/sh

# デバッグ用に、どのスクリプトが実行されているかを出力
echo "==> [run-debug.sh] Executing..."

# ポート2345を解放する
# プロセスが見つからなくてもエラーにならないように `|| true` を付加
echo "==> [run-debug.sh] Attempting to free port 2345..."
lsof -t -i:2345 | xargs kill -9 || true

# 念のため少し待つ (任意)
# sleep 0.1

# dlvコマンドを実行する
# 実行するバイナリのパスは、カレントディレクトリ(/app)からの相対パスで指定
echo "==> [run-debug.sh] Starting Delve on port 2345..."
dlv exec --headless --listen=:2345 --accept-multiclient --continue ./cmd/tmp/main