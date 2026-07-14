# 開発推奨コマンド (Windows PowerShell)

本プロジェクトの開発において使用する推奨コマンド一覧です。環境は Windows 11 PowerShell を前提としています。

## ビルド・実行・開発 (Taskfile 経由)
本プロジェクトでは `Taskfile.yml` が定義されており、`task` コマンドを使用して主要な開発操作を行います。

- **開発モードの起動 (ホットリロード)**:
  ```powershell
  task dev
  ```
  （内部的に `wails3 dev -config ./build/config.yml -port 9245` を実行）

- **アプリケーションのビルド**:
  ```powershell
  task build
  ```

- **アプリケーションの実行**:
  ```powershell
  task run
  ```

- **Dockerを用いたクロスコンパイル環境のセットアップ**:
  ```powershell
  task setup:docker
  ```

- **サーバーモード (GUIなし、HTTPサーバーのみ) のビルドと実行**:
  ```powershell
  task build:server
  task run:server
  ```

## フロントエンド単体のコマンド (frontend ディレクトリ)
- **依存関係のインストール**:
  ```powershell
  cd frontend
  npm install
  ```
- **Svelte・TypeScriptの型チェック**:
  ```powershell
  cd frontend
  npm run check
  ```
- **フロントエンドのビルド**:
  ```powershell
  cd frontend
  npm run build
  ```

## バックエンド単体のコマンド
- **Goモジュールのクリーンアップと同期**:
  ```powershell
  go mod tidy
  ```
- **Goテストの実行**:
  ```powershell
  go test ./...
  ```
- **Goコードの自動フォーマット**:
  ```powershell
  go fmt ./...
  ```

## Windows PowerShell 環境におけるユーティリティコマンドの規則
Linux のエイリアスではなく、必ず正規の PowerShell コマンドレットを使用してください。
- ファイル/ディレクトリ一覧: `Get-ChildItem` (不可: `ls`)
- ファイル/ディレクトリ削除: `Remove-Item` (不可: `rm`) - 破壊的変更には `-WhatIf` を付けて確認を推奨。
- ファイル/ディレクトリコピー: `Copy-Item` (不可: `cp`)
- ファイル/ディレクトリ移動: `Move-Item` (不可: `mv`)
- パターン検索: `Select-String` (不可: `grep`)
- 新規ファイル作成: `New-Item -ItemType File` (不可: `touch`)
- 新規ディレクトリ作成: `New-Item -ItemType Directory -Force` (不可: `mkdir`)
- ファイル内容の出力: `Get-Content` (不可: `cat`)
- 現在のパス取得: `Get-Location` (不可: `pwd`)
