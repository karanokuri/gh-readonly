# gh-readonly

書き込み操作を拒否する [GitHub CLI](https://cli.github.com/) 拡張。

Claude Code などの AI エージェントが `gh` コマンドを実行すると、意図せず Issue を作成したり PR をマージしたりする可能性がある。`gh-readonly` はローカルリバースプロキシで `gh` をラップし、`POST`・`PUT`・`PATCH`・`DELETE` リクエストを HTTP 403 で遮断する。

## 仕組み

1. 自己署名証明書でローカル HTTPS プロキシを起動
2. `HTTPS_PROXY` と `SSL_CERT_FILE` をプロキシに向けて実際の `gh` を起動
3. `GET`・`HEAD` リクエストは通過、`POST`・`PUT`・`PATCH`・`DELETE` は HTTP 403 で拒否

## 必要環境

- Linux（Go プログラムが `SSL_CERT_FILE` を参照するのは Linux のみ）
- [GitHub CLI](https://cli.github.com/) インストール済み・認証済み

## インストール

### gh extension install（推奨）

```bash
gh extension install karanokuri/gh-readonly
```

### GitHub Releases から手動インストール

```bash
curl -L https://github.com/karanokuri/gh-readonly/releases/latest/download/gh-readonly_linux-amd64 \
  -o gh-readonly
chmod +x gh-readonly
mkdir -p ~/.local/share/gh/extensions/gh-readonly
mv gh-readonly ~/.local/share/gh/extensions/gh-readonly/gh-readonly
```

### ソースから

```bash
git clone https://github.com/karanokuri/gh-readonly.git
cd gh-readonly
go build -o gh-readonly .
mkdir -p ~/.local/share/gh/extensions/gh-readonly
cp gh-readonly ~/.local/share/gh/extensions/gh-readonly/gh-readonly
```

## 使い方

```bash
gh readonly <gh のコマンド>
```

### 例

```bash
# 読み取り操作 — 通常通り動作
gh readonly repo view
gh readonly pr list
gh readonly issue list --repo cli/cli

# 書き込み操作 — 遮断
gh readonly issue create --title "test"
# gh-readonly: denied — write operation (POST /repos/owner/repo/issues) is not allowed.
# Use 'gh' directly (requires user approval).
```

## Claude Code との連携

プロジェクトの `CLAUDE.md` に追記する：

```markdown
GitHub CLI コマンドは gh の代わりに gh readonly を使うこと。
```
