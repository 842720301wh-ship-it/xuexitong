# 上传到 GitHub

## 推荐方式：上传整理好的源码

在项目根目录执行，电脑只需已安装 Node.js 和 npm：

```sh
npm run export:github
```

脚本生成 `release/github-upload/`，其中包含可上传的源码、配置、公开文档与预览图。第三方源码、作者注释和已有许可文件一并保留。脚本只导出文件，不连接或推送到 GitHub。

再次执行会替换原有的 `release/github-upload/`。请在项目原始源码中修改内容，再重新导出；不要在上传目录中保存个人资料。导出采用固定目录白名单和文件排除规则，新增顶层配置或文档时，需要相应更新 `scripts/export-github.mjs`。该脚本不会自动识别源码中手动写入的真实密钥，提交前仍需检查自己的修改。

在 GitHub 仓库的文件上传页面，上传该目录**里面的文件和文件夹**，保留 `.gitignore` 等隐藏文件。完成后，仓库首页应直接出现：

```text
android/
backend/
docs/
frontend/
scripts/
.gitignore
package-lock.json
package.json
README.md
```

不要让仓库多嵌套一层 `github-upload/`，也不要将整个 `release/` 上传。

## 通过 Git 上传

建议将导出目录的内容复制到单独的 Git 工作目录，在该工作目录中初始化仓库并检查清单：

```sh
git init
git status --short --untracked-files=all
git add --dry-run .
```

检查清单中没有个人资料、密钥、依赖和构建产物后，再执行：

```sh
git add .
git commit -m "Prepare source for GitHub"
git branch -M main
```

在 GitHub 页面创建空仓库后，按页面给出的远程地址设置 `origin` 并推送。若目标仓库已有提交，请先克隆已有仓库，再把导出目录内容复制进去，检查差异后提交；不要用强制推送覆盖历史。

也可以直接在项目根目录管理 Git，但必须保留 `.gitignore` 并检查提交清单。请将 Git 历史保存在独立工作目录或项目根目录；如果上传目录中已存在 `.git`，导出脚本会拒绝覆盖，以保留仓库历史。

## 哪些目录可以上传

| 可以上传 | 需要排除的内容 |
| --- | --- |
| `frontend/` | `node_modules/`、`dist/`、`*.tsbuildinfo`、真实 `.env*` |
| `backend/` | 二进制、生成的 `cmd/android/dist/`、本机数据与密钥 |
| `android/` | `.gradle/`、所有 `build/`、`local.properties`、生成的 `jniLibs/`、签名文件 |
| `scripts/` | 本机输出、日志、缓存及个人配置 |
| `docs/usage.md`、`docs/github-upload.md`、`docs/screenshots/` | 不包含原始研究、实机记录与设计过程目录 |
| 根目录 `README.md`、`.gitignore`、`package.json`、`package-lock.json` | 不包含个人环境配置 |

**完整的原始目录不能直接整体上传。** 根目录的 `.gitignore` 为 Git 提交排除以下内容；网页拖拽上传不会执行这些规则：

- `data/`：账号、API Key、加密配置和对应密钥。
- `tool/`、`work/`、`.tools/`：原始 APK、工具与研究材料。
- `.cache/`、`node_modules/`、`dist/`：本机缓存、签名密钥、依赖与构建产物。
- `docs/research/`、`docs/testing/`、`docs/design/`：研究、实机测试记录和未逐一检查的截图。
- `release/`：导出目录与 APK 成品，均不作为源码目录整体提交。
- 环境变量、Cookie、私钥、签名文件、日志、HAR / PCAP 等抓包文件。

`.gitignore` 不影响已经被 Git 跟踪的文件。若之前已提交个人信息，新增忽略规则并不能从历史记录中移除它们；如果提交过真实密钥，应更换密钥并处理相关历史。

## 安装包单独发布

本项目当前 Android 产物为 `release/Android/天天向上-0.2.0-debug.apk`。如需向用户提供下载，可将确认过的安装包单独作为 GitHub Release 附件，附上版本说明；不要将依赖缓存、签名密钥或原始 `tool/base.apk` 一起发布。

当前包使用 debug 签名。`.cache/android-debug.keystore` 留在本机；其他电脑会生成不同的 debug 密钥，构建出的 APK 可能无法覆盖安装旧包。

[返回项目首页](../README.md)
