# m3bridge

Microsoft Graph APIを使用してメールを送信するローカルSMTPサーバです。OAuth 2.0認証を使用し、SMTPプロトコル経由で受信したメールをMicrosoft Graphで送信します。

## インストール

### Docker（推奨）

Go環境なしで動作します。

```bash
docker pull ghcr.io/canaria-computer/m3bridge:latest
```

### go install

```bash
go install github.com/canaria-computer/m3bridge@latest
```

### ソースからビルド

```bash
git clone https://github.com/canaria-computer/m3bridge.git
cd m3bridge
go build -o m3bridge
```

## 使い方

### Docker経由で使う

#### 1. 初回認証

ブラウザ操作が必要なため、ホスト上で実行します。

```bash
docker run --rm -it \
  -v "${HOME}/.m3bridge:/root/.m3bridge" \
  ghcr.io/canaria-computer/m3bridge:latest auth
```

ブラウザで表示されるURLを開き、Microsoftアカウントで認証します。

#### 2. サーバ起動（docker compose）

```bash
docker compose up -d
```

または `docker run` で直接起動:

```bash
docker run -d \
  --name m3bridge \
  -p 2525:2525 \
  -v "${HOME}/.m3bridge:/root/.m3bridge" \
  ghcr.io/canaria-computer/m3bridge:latest
```

### バイナリで使う

#### 1. 初回認証

```bash
m3bridge auth
```

認証をテストする場合:

```bash
m3bridge auth --test
```

#### 2. SMTPサーバ起動

```bash
m3bridge serve
```

デフォルトでは `localhost:2525` で起動します。ポートを変更する場合:

```bash
m3bridge serve --port 587
```

起動時に表示される接続情報を確認してください:

```
=== SMTP接続情報 ===
サーバ: localhost:2525
ユーザー名: m3bridge
パスワード: <自動生成されたパスワード>
セキュリティ: なし（平文）
設定ファイル: /home/user/.m3bridge/config.json
=====================
```

### メールクライアントの設定

- **SMTPサーバ**: localhost
- **ポート**: 2525（または指定したポート）
- **セキュリティ**: なし / STARTTLS無効
- **認証**: PLAIN
- **ユーザー名**: m3bridge
- **パスワード**: 起動時に表示されたパスワード

#### Thunderbirdの例

1. アカウント設定 → 送信(SMTP)サーバ
2. 新しいサーバを追加
3. 上記の設定を入力

## コマンド

### auth

Microsoft Graphで認証し、アクセストークンを取得します。

```bash
m3bridge auth [flags]
```

**フラグ:**

- `--test`: 認証後にユーザー情報を取得してテスト

### serve

SMTPサーバを起動します。

```bash
m3bridge serve [flags]
```

**フラグ:**

- `-p, --port int`: SMTPサーバのポート番号（デフォルト: 2525）

### グローバルフラグ

- `--config string`: 設定ファイルパス
- `--log-level string`: ログレベル（debug, info, warn, error）（デフォルト: info）

## トラブルシューティング

### トークンが期限切れ

トークンは自動的に再取得されますが、手動でキャッシュをクリアする場合:

```bash
rm ~/.m3bridge/token_cache.json
m3bridge auth
```

### SMTP認証エラー

正しいユーザー名とパスワードを使用しているか確認してください。設定は以下で確認できます:

```bash
cat ~/.m3bridge/config.json
```

### メール送信失敗

ログレベルをdebugに設定して詳細情報を確認:

```bash
m3bridge serve --log-level debug
```

## 関連リンク

- [Microsoft Graph API](https://learn.microsoft.com/graph/)
- [go-smtp](https://github.com/emersion/go-smtp)
- [Cobra CLI](https://github.com/spf13/cobra)
