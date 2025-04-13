@echo off
REM コードページをUTF-8（65001）に設定
chcp 65001 >nul

REM バッチファイルが置かれているディレクトリを取得
set WSL_DIR=%~dp0

REM ログファイルのパスを指定
set LOG_FILE=%WSL_DIR%logs\docker_build_and_aws_push.txt

REM ログファイルが存在する場合削除
if exist "%LOG_FILE%" del "%LOG_FILE%"

REM 実行開始時刻をログに記録
echo Execution started at %date% %time% >> "%LOG_FILE%"
echo -------------------------------------------------- >> "%LOG_FILE%"

REM 現在のディレクトリパスをログに出力
echo Batch file directory: %WSL_DIR% >> "%LOG_FILE%"

echo "■AWSログイン" >> "%LOG_FILE%"
aws ecr get-login-password --region ap-northeast-1 | docker login --username AWS --password-stdin 990606419933.dkr.ecr.ap-northeast-1.amazonaws.com >> "%LOG_FILE%"
echo "■ビルド" >> "%LOG_FILE%"
@REM docker build -t react-frontend -f Dockerfile.prod . >> "%LOG_FILE%"
docker compose --file docker/docker-compose.prod.yml --env-file docker/env/.env.prod build --no-cache >> "%LOG_FILE%"
echo "■タグつけ" >> "%LOG_FILE%"
docker tag test/go_4_vocab_keep:latest 990606419933.dkr.ecr.ap-northeast-1.amazonaws.com/test/go_4_vocab_keep:latest >> "%LOG_FILE%"
echo "■プッシュ" >> "%LOG_FILE%"
docker push 990606419933.dkr.ecr.ap-northeast-1.amazonaws.com/test/go_4_vocab_keep:latest >> "%LOG_FILE%"

REM 実行完了時刻をログに記録
echo Execution finished at %date% %time% >> "%LOG_FILE%"
echo -------------------------------------------------- >> "%LOG_FILE%"

REM 実行結果を表示
echo Docker control script executed in WSL

