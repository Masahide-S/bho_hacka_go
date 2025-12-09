# devmon

ローカル開発環境のサービス（PostgreSQL, Docker, Node.js, Pythonなど）の状態を一括でモニタリングするためのCLIツールです。

## ✨ 特徴

現在、以下の情報を自動検出して表示します：

  * **Docker**: 実行中のコンテナ数、CPU/メモリ使用率、イメージサイズ、マウントポイント
  * **PostgreSQL**: 稼働状況、ポート番号、データベース一覧（サイズ、作成日、最終接続日時）
  * **Node.js**: プロセス検知、実行中のプロジェクト名（`package.json`から取得）、稼働時間、CPU/メモリ使用量
  * **Python**: プロセス検知、フレームワーク判定（Django, Flask, Jupyter, FastAPI等）、稼働時間
  * **ポート情報**: 現在リッスンしているポートと対応プロセスの一覧

## 📦 前提条件 (Prerequisites)

このツールは内部でOSのコマンドを使用するため、以下のコマンドがパス（PATH）に通っている環境（主にmacOSまたはLinux）で動作します。

  * **Go**: 1.25以上
  * **Docker CLI**: `docker` コマンド
  * **lsof**: ポートやプロセスのカレントディレクトリ特定に使用 (`sudo apt install lsof` 等が必要な場合があります)
  * **pgrep / ps**: プロセス検索用
  * **psql**: PostgreSQLの詳細情報を取得する場合に必要（クライアントツール）

> **注意**: Windows環境では、WSL2上であれば動作する可能性がありますが、ネイティブ環境ではコマンド体系が異なるため動作しない可能性があります。

## 🚀 インストールとビルド

### 1\. リポジトリのクローン

```bash
git clone https://github.com/Masahide-S/bho_hacka_go.git
cd bho_hacka_go
```

### 2\. 依存関係の解決

```bash
go mod tidy
```

### 3\. ビルド

実行可能なバイナリファイルを作成します。

```bash
go build -o devmon main.go
```

## 💻 使い方 (Usage)

### 方法 A: ビルドしたバイナリを実行する

ビルドが完了していれば、以下のコマンドで実行できます。

```bash
./devmon
```

### 方法 B: ソースコードから直接実行する

開発中など、ビルドせずに実行したい場合は以下のようにします。

```bash
go run main.go
```

### 実行結果イメージ

コマンドを実行すると、以下のように現在の環境のステータスが表示されます。

```text
=== Local Development Monitor ===

監視機能を実装中...
✓ PostgreSQL: 実行中 [:5432] | 稼働: 2d 10h
  - my_app_db (54 MB) | 作成: 2024-01-01 | 最終接続: 10分前

✓ Docker: 2個のコンテナ
  - web-app [:3000] | Up 2 hours | CPU: 0.5% | メモリ: 120MB
    └─ Image: node:18-alpine (180MB)
  - db-redis [:6379] | Up 2 hours
    └─ Mount: /home/user/project/data

✓ Node.js: 実行中 [:3000]
  └─ /home/user/projects/frontend
     (package.json: my-frontend) | 稼働: 01:23:45 | CPU: 0.1% | メモリ: 85 MB

✓ Python: 実行中 [:8000]
  └─ /home/user/projects/backend
     (FastAPI) | 稼働: 00:45:12 | CPU: 1.2% | メモリ: 60 MB

使用中のポート:
  :3000 - node
  :5432 - postgres
  :8000 - python
```

## 🛠️ トラブルシューティング

  * **ポート情報が表示されない**: `lsof` コマンドがインストールされているか確認してください。
  * **PostgreSQLの詳細が出ない**: ローカルで `psql` コマンドが使用可能で、現在のユーザーから `postgres` データベースへパスワードなし（または `.pgpass` 設定済み）でアクセスできる必要があります。
  * **Docker情報が出ない**: Docker DesktopまたはDocker Engineが起動しているか確認してください。

## 📜 ライセンス

MIT License - 詳細は [LICENSE](https://www.google.com/search?q=LICENSE) ファイルをご確認ください。