@echo off
REM ==========================================
REM バッチファイルの目的:
REM WSL内でDocker関連スクリプトを実行し、ログを記録する
REM ==========================================

REM コードページをUTF-8（65001）に設定
chcp 65001

REM 1. バッチファイルが存在するディレクトリを取得
SET "CURRENT_DIR=%~dp0"
REM 最後のバックスラッシュ（\）を削除
SET "CURRENT_DIR=%CURRENT_DIR:~0,-1%"

REM 2. 親ディレクトリを取得
FOR %%I IN ("%CURRENT_DIR%") DO SET "PARENT_DIR=%%~dpI"
REM 最後のバックスラッシュ（\）を削除
SET "PARENT_DIR=%PARENT_DIR:~0,-1%"

REM 3. 変数 DEV を定義
SET "DEV=local"

REM 4. バッチファイルのディレクトリに移動
CD /D "%CURRENT_DIR%"

REM 5. WindowsパスをWSLパスに変換
FOR /F "usebackq tokens=*" %%A IN (`wsl wslpath "%CURRENT_DIR%"`) DO SET "WSL_CURRENT_DIR=%%A"
FOR /F "usebackq tokens=*" %%A IN (`wsl wslpath "%PARENT_DIR%"`) DO SET "WSL_PARENT_DIR=%%A"

REM 5.1 ログ出力
echo CURRENT_DIR: %CURRENT_DIR%
echo PARENT_DIR: %PARENT_DIR%
echo WSL_CURRENT_DIR: %WSL_CURRENT_DIR%
echo WSL_PARENT_DIR: %WSL_PARENT_DIR%
echo DEV: %DEV%

REM 6. WSLでスクリプトを実行し、親ディレクトリとDEVを引数として渡す
wsl bash -c "cd \"%WSL_CURRENT_DIR%\" && chmod +x ./run_docker_script_in_wsl.sh && ./run_docker_script_in_wsl.sh \"%WSL_PARENT_DIR%\" \"%DEV%\" > \"%WSL_CURRENT_DIR%/script_output.log\" 2> \"%WSL_CURRENT_DIR%/script_error.log\""
SET "ERRORLEVEL_WSL=%ERRORLEVEL%"

REM 7. WSLスクリプトの実行結果を確認
IF %ERRORLEVEL_WSL% NEQ 0 (
    echo ERROR: WSLスクリプトの実行に失敗しました。エラーコード: %ERRORLEVEL_WSL%
    echo 詳細については、以下のログファイルを確認してください。
    echo   - %CURRENT_DIR%\script_output.log
    echo   - %CURRENT_DIR%\script_error.log
    EXIT /B %ERRORLEVEL_WSL%
) ELSE (
    echo WSLスクリプトは正常に実行されました。
)

REM ================================================
REM バッチファイルの終了
REM ================================================
EXIT /B