# Matchmaking Probe Loader：优化版运行组件

[优化版项目说明](../../../README.md) · [过滤器运行说明](../matchmaking_row_filter_dll/README.md)

文档更新：2026-10-08（Asia/Shanghai）。本文件属于优化版预构建载荷；根目录已同步宿主优化，但两处 EXE 同级数据和说明路径独立。

## 用途与架构

`L4D2MatchmakingProbeLoader.exe` 是随包的 x86 Loader，用于在本机 32 位 L4D2 进程中加载过滤 DLL。当前发布宿主为 x64，Loader/DLL 保持 x86。

本仓库只包含预构建 EXE，不包含 Loader 的 Go 源码、独立 go.mod、rsrc 构建配置或 `run_loader_admin.ps1`。`package-portable.ps1` 检查并嵌入现有文件，不重新编译 Loader。旧独立研究项目的编译命令不是此目录的构建入口。

## 融合版运行位置

相对实际启动的 `L4D2_MIX.exe`：

```text
data/
├─ matchmaking_probe_loader/L4D2MatchmakingProbeLoader.exe
└─ row-filter/
   ├─ matchmaking_row_filter.dll
   ├─ load_row_filter_admin.ps1
   ├─ launch_row_filter_early_admin.ps1
   └─ matchmaking_row_filter.log
```

推荐通过“组服务器过滤”页操作。在 EXE 所在目录也可调用：

```powershell
& .\data\row-filter\load_row_filter_admin.ps1
& .\data\row-filter\launch_row_filter_early_admin.ps1
```

前者面向已运行的游戏；后者通过 Steam 启动并等待游戏进程最多 120 秒，发现进程后尽早加载。若游戏已运行，后者使用现有进程。脚本从自身目录定位 DLL，并从相邻 `matchmaking_probe_loader` 目录定位 Loader；按提示处理管理员权限。

日志位于 `data/row-filter/matchmaking_row_filter.log`。本仓库没有独立 probe DLL，不承诺生成历史研究项目的 `matchmaking_probe.log`。

## 优化版载荷维护

- 固定文件参与构建清单和 24 小时元数据缓存；缺失、元数据变化、版本变化、缓存损坏或到期会触发 SHA-256 校验与修复。
- `L4D2_MIX_VERIFY_PAYLOAD=1` 每次完整校验固定载荷；已有可编辑规则、模式和日志保留。
- Loader 载荷准备与默认连跳页初始化重叠；过滤器界面在默认页初始化完成后启动。Loader 不因打开窗口自动加载到游戏，需要用户执行相应操作。
- `L4D2_MIX_TRACE_STARTUP=1` 的 `data/startup-traces` 只用于界面启动诊断；游戏加载结果仍以过滤器日志为准。

## 排查与分发

先正常启动融合 EXE，确认自动释放的目录齐全。主菜单已有旧行时可重新启动并使用提前加载流程；加载成功不等于清空此前 UI 缓存。游戏升级后核对日志、版本和架构，不套用旧模块偏移。

便携 ZIP 只含宿主 EXE 和 README.txt，首次运行释放 Loader。升级前保留个人 data，公开分发排除日志、规则和部署状态。
