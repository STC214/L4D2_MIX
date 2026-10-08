# Matchmaking Probe Loader：原版运行组件

[原版项目说明](../../../README.md) · [过滤器运行说明](../matchmaking_row_filter_dll/README.md)

文档更新：2026-10-08（Asia/Shanghai）。本文件属于根目录原版的预构建运行载荷；优化版的缓存、调度和首帧修复见仓库中的 `L4D2_MIX_optimized`，不由这个 Loader 实现。

## 用途与架构

`L4D2MatchmakingProbeLoader.exe` 是随包的 x86 Loader，用于在本机 32 位 L4D2 进程中加载过滤 DLL。宿主当前发布为 x64，这不意味着 Loader/DLL 应改成 x64。

本仓库只包含该预构建 EXE，不包含 Loader 的 Go 源码、独立 go.mod、rsrc 构建配置或 `run_loader_admin.ps1`。项目打包脚本检查并嵌入现有文件，不重新编译 Loader。旧独立研究项目的构建命令不是本目录的构建入口。

## 融合版运行位置

在实际启动的 `L4D2_MIX.exe` 同级目录中，首次运行会创建：

```text
data/
├─ matchmaking_probe_loader/L4D2MatchmakingProbeLoader.exe
└─ row-filter/
   ├─ matchmaking_row_filter.dll
   ├─ load_row_filter_admin.ps1
   ├─ launch_row_filter_early_admin.ps1
   └─ matchmaking_row_filter.log
```

推荐通过融合工具的“组服务器过滤”页操作。也可在 EXE 所在目录调用已释放的管理脚本：

```powershell
# 已运行的游戏
& .\data\row-filter\load_row_filter_admin.ps1

# 启动 Steam 游戏并尽早加载
& .\data\row-filter\launch_row_filter_early_admin.ps1
```

两个脚本从自身目录定位 DLL，并从相邻 `matchmaking_probe_loader` 目录定位 Loader。提前启动脚本等待游戏进程最多 120 秒；若游戏已运行，使用现有进程。运行时按脚本提示处理管理员权限。

过滤器日志位于 `data/row-filter/matchmaking_row_filter.log`。本仓库没有独立 probe DLL，因此这里不承诺生成旧研究项目的 `matchmaking_probe.log`。

## 排查与升级

- Loader 或 DLL 缺失：先正常启动融合 EXE，让它释放载荷；只复制源码目录下某个文件不是完整运行布局。
- 主菜单已有旧服务器行：重新启动并使用提前加载流程，加载成功不等于清空此前的 UI 缓存。
- 游戏升级后行为改变：先查看过滤日志、版本和架构；历史模块偏移不保证适用于当前游戏版本。
- 公开分发只带项目生成的 EXE/便携包，不附个人规则、日志或游戏路径。升级前保留 EXE 同级 data。

Loader 的游戏加载动作与宿主启动性能是两个阶段；优化版 `data/startup-traces` 记录界面启动，不替代过滤器日志。
