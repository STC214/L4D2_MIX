# Matchmaking Row Filter：优化版运行载荷

[优化版项目说明](../../../README.md) · [Loader 运行说明](../matchmaking_probe_loader/README.md)

文档更新：2026-10-08（Asia/Shanghai）。本文件属于优化版预构建载荷；根目录已同步宿主优化，但两处 EXE 同级数据和说明路径独立。

## 文件与模式

`matchmaking_row_filter.dll` 是 x86 DLL，配合同架构 Loader 在 32 位游戏中运行。主项目使用预构建文件；DLL 独立源码、`build_msvc_x86.bat` 和历史研究工具未包含在本仓库。

随包 `row_filter_mode.txt` 当前为 **`steam_serverlist_drop`**，这是当前分发配置。旧文档中的 `neutralize_keyword_then_late_skip` 等实验模式和模块偏移，不是当前默认值或跨游戏版本保证。实际加载模式与结果请核对运行日志。

| 可编辑文件 | 用途 |
| --- | --- |
| `blocked_keywords.txt` | 关键字列表 |
| `blocked_connectstrings.txt` | 手工永久地址列表 |
| `learned_connectstrings.txt` | 学习地址列表 |
| `auto_derived_connectstrings.txt` | 运行时派生地址列表 |
| `row_filter_mode.txt` | 当前模式 |

TXT 是运行配置，不是本次更新的文档。规则匹配、大小写与派生行为以对应 DLL 的日志为准；文档更新不改变规则或 DLL。

## 运行布局与操作

融合 EXE 首次运行释放过滤文件到 EXE 同级 `data/row-filter`，Loader 在相邻 `data/matchmaking_probe_loader`。界面、DLL 和脚本共享这份规则。源码的 `payload/runtime` 是嵌入载荷，用户应在运行目录或界面编辑配置。

在实际 EXE 所在目录执行：

```powershell
& .\data\row-filter\load_row_filter_admin.ps1
& .\data\row-filter\launch_row_filter_early_admin.ps1
Get-Content .\data\row-filter\row_filter_mode.txt
Get-Content .\data\row-filter\matchmaking_row_filter.log -Tail 80
```

前两个脚本分别用于已运行游戏和提前启动/加载。提前脚本通过 Steam 启动并等待进程最多 120 秒；加载成功不保证清除此前 UI 缓存。规则修改后按界面提示重新加载或重启。优先通过融合工具的“组服务器过滤”页操作。

## 配置保留、缓存与恢复

固定 DLL/脚本/默认种子参与构建清单与 24 小时元数据缓存；缺失、大小/修改时间变化、版本变化、缓存损坏或到期时重新 SHA-256 校验并修复。设置 `L4D2_MIX_VERIFY_PAYLOAD=1` 可每次完整校验固定载荷。已有可编辑规则、模式和日志不被应用更新覆盖。

`config_defaults` 有四份默认 TXT 和 `default_filter_config.json` 说明。主动恢复会**覆盖四份当前规则**，先备份再执行：

```powershell
& .\data\row-filter\restore_default_configs.ps1
```

该脚本不重置模式文件或日志。JSON 种子不是“保存配置”的输出文件。完整载荷校验也不覆盖已有可编辑规则，这与用户主动恢复默认不同。

## 启动诊断与维护

- 过滤器页面在默认连跳页初始化完成后自动启动，不依赖点击标签。
- 页面初始化仍读取规则与日志尾部，宿主的 `L4D2MixReady` 和 `L4D2MixAttached` 标记与游戏 DLL 加载成功不是同一件事。
- `L4D2_MIX_TRACE_STARTUP=1` 记录界面启动至 `data/startup-traces`；过滤效果看 `data/row-filter/matchmaking_row_filter.log`。
- 深色容器、首帧重绘和托盘恢复不重新加载游戏 DLL，也不改变后台任务生命周期。
- 先核对日志中的模式、版本与错误，不将历史模块偏移用于兼容性保证。

便携 ZIP 不含个人 data，首次运行自动创建运行目录；升级保留原 data，公开分发排除个人规则、日志和部署状态。本组件不是 Workshop VPK，使用时遵守服务器和平台规则。
