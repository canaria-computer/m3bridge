# Microsoft Graph SMTP Server

Microsoft Graph APIを使用してメールを送信するローカルSMTPサーバです。OAuth 2.0認証を使用し、SMTPプロトコル経由で受信したメールをMicrosoft Graphで送信します。

## インストール

```bash
go install github.com/yourusername/msgraph-smtp@latest
```

または、ソースからビルド:

```bash
git clone https://github.com/yourusername/msgraph-smtp.git
cd msgraph-smtp
go build -o msgraph-smtp
```

## 使い方

### 1. 初回認証

```bash
msgraph-smtp auth
```

ブラウザで表示されるURLを開き、Microsoftアカウントで認証します。

認証をテストする場合:

```bash
msgraph-smtp auth --test
```

### 2. SMTPサーバ起動

```bash
msgraph-smtp serve
```

デフォルトでは `localhost:2525` で起動します。ポートを変更する場合:

```bash
msgraph-smtp serve --port 587
```

起動時に表示される接続情報を確認してください:

```
=== SMTP接続情報 ===
サーバ: localhost:2525
ユーザー名: msgraph
パスワード: <自動生成されたパスワード>
セキュリティ: なし（平文）
設定ファイル: /home/user/.msgraph-smtp/config.json
=====================
```

### 3. メールクライアントの設定

お使いのメールクライアントで以下の設定を行います:

- **SMTPサーバ**: localhost
- **ポート**: 2525 (または指定したポート)
- **セキュリティ**: なし / STARTTLS無効
- **認証**: PLAIN
- **ユーザー名**: msgraph
- **パスワード**: 起動時に表示されたパスワード

#### Thunderbirdの例

1. アカウント設定 → 送信(SMTP)サーバ
2. 新しいサーバを追加
3. 上記の設定を入力

## コマンド

### auth

Microsoft Graphで認証し、アクセストークンを取得します。

```bash
msgraph-smtp auth [flags]
```

**フラグ:**

- `--test`: 認証後にユーザー情報を取得してテスト

### serve

SMTPサーバを起動します。

```bash
msgraph-smtp serve [flags]
```

**フラグ:**

- `-p, --port int`: SMTPサーバのポート番号 (デフォルト: 2525)

### グローバルフラグ

- `--config string`: 設定ファイルパス
- `--log-level string`: ログレベル (debug, info, warn, error) (デフォルト: info)

## トラブルシューティング

### トークンが期限切れ

トークンは自動的に再取得されますが、手動でキャッシュをクリアする場合:

```bash
rm ~/.msgraph-smtp/token_cache.json
msgraph-smtp auth
```

### SMTP認証エラー

正しいユーザー名とパスワードを使用しているか確認してください。設定は以下で確認できます:

```bash
cat ~/.msgraph-smtp/config.json
```

### メール送信失敗

ログレベルをdebugに設定して詳細情報を確認:

```bash
msgraph-smtp serve --log-level debug
```

## 関連リンク

- [Microsoft Graph API](https://learn.microsoft.com/graph/)
- [go-smtp](https://github.com/emersion/go-smtp)
- [Cobra CLI](https://github.com/spf13/cobra)
