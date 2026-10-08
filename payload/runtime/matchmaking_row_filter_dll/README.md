# Matchmaking Row Filter：根目录运行载荷

[根目录项目说明](../../../README.md) · [Loader 运行说明](../matchmaking_probe_loader/README.md)

文档更新：2026-10-08（Asia/Shanghai）。这里是根目录的预构建运行载荷；根目录宿主已同步优化副本的启动缓存、调度和首帧修复。Loader/DLL 本体仍为仓库内原有预构建文件，宿主优化不代表修改它们的二进制。

## 文件与模式

`matchmaking_row_filter.dll` 为 x86，配合同架构 Loader 在 32 位游戏进程中运行。DLL 独立源码、`build_msvc_x86.bat` 和历史研究工具未包含在本仓库；主项目打包嵌入现有运行文件，不重新编译 DLL。

随包 `row_filter_mode.txt` 当前内容是 **`steam_serverlist_drop`**。它是当前分发配置；旧文档中的 `neutralize_keyword_then_late_skip` 等研究模式或模块偏移，不应当作当前默认值或跨版本保证。实际加载模式及结果请查看运行日志。

可编辑规则：

| 文件 | 用途 |
| --- | --- |
| `blocked_keywords.txt` | 关键字列表 |
| `blocked_connectstrings.txt` | 手工永久地址列表 |
| `learned_connectstrings.txt` | 学习地址列表 |
| `auto_derived_connectstrings.txt` | 运行时派生地址列表 |
| `row_filter_mode.txt` | 当前模式 |

这些 TXT 是运行数据，不是 Markdown 文档。关键字匹配的 ASCII 大小写行为、派生和匹配结果以对应 DLL 的日志为准；文档更新不修改规则或 DLL。

## 使用实际运行目录

融合 EXE 首次运行把过滤器运行文件释放到 EXE 同级 `data/row-filter`；Loader 位于相邻 `data/matchmaking_probe_loader`。管理界面、DLL 和脚本共同使用这份规则，源码 `payload/runtime` 是打包载荷，不是用户应编辑的运行目录。

在实际 EXE 所在目录执行：

```powershell
# 对已运行的游戏加载
& .\data\row-filter\load_row_filter_admin.ps1

# 启动游戏后尽早加载
& .\data\row-filter\launch_row_filter_early_admin.ps1

# 查看实际模式及日志
Get-Content .\data\row-filter\row_filter_mode.txt
Get-Content .\data\row-filter\matchmaking_row_filter.log -Tail 80
```

优先使用融合工具的过滤器页。提前加载脚本启动 Steam 游戏并等待进程最多 120 秒；加载过晚时，已有 UI/服务器列表缓存可能仍保留旧行。改变规则后按界面提示重新加载或重启；仅编辑文件不代表已运行 DLL 即时采用新值。

## 恢复默认规则

`config_defaults` 提供四份 TXT 种子和 `default_filter_config.json` 说明。恢复操作会**覆盖**四份当前规则，执行前先备份：

```powershell
& .\data\row-filter\restore_default_configs.ps1
```

该脚本不重置 `row_filter_mode.txt`，也不清空日志；JSON 种子不是界面“保存配置”的输出文件。应用更新保留已有规则、模式和日志，但用户主动恢复默认会覆盖规则。

## 日志、兼容性与分发

日志为 `data/row-filter/matchmaking_row_filter.log`。先核对加载是否成功、模式、规则数量、版本和错误，再判断过滤效果。历史硬编码模块偏移属于旧研究记录，不作为当前游戏版本兼容性说明。

本组件不是 Workshop VPK。遵守服务器与平台规则，并确认本机运行文件可信。升级前备份 data；公开分发不带个人服务器规则、日志和部署记录。
