@echo off
REM ==========================================
REM バッチファイルの目的:
REM WSL内でDocker関連スクリプトを実行し、ログを記録する
REM ==========================================

REM コードページをUTF-8（65001）に設定
chcp 65001 >nul

REM スクリプトを実行したディレクトリを取得
for /f "tokens=*" %%i in ('docker ps -a -q') do docker rm -f %%i
