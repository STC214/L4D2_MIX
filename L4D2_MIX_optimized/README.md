# L4D2 MIX 优化版

[仓库首页](../README.md) · [Loader 说明](payload/runtime/matchmaking_probe_loader/README.md) · [过滤器说明](payload/runtime/matchmaking_row_filter_dll/README.md)

文档更新：2026-10-08（Asia/Shanghai）。命令默认在 `L4D2_MIX_optimized` 目录执行。根目录已同步本页的优化与后续修复，并作为推荐开发入口；本目录保留同步的软件副本。

一个统一的 Go + Win32 控制台，将以下三个现有工具放进同一个窗口：

- `Left4Dead2-Autobhop_VPK_W`：窗口选择、偏移配置、自动探测、启动/停止和实时状态。
- `L4D2RowFilterManager.exe`：规则编辑、恢复默认、启动并注入、注入运行中的游戏、日志与组件目录。
- `L4D2_MOD_JOIN`：VPK 动态扫描分类、冲突处理、分类合并、武器音效音量处理、安全部署和还原。分类输出包使用 `【01】UI_HUD.vpk`、`【02】Survivors.vpk` 这样的括号编号格式。

## 使用

运行：

```text
dist\L4D2_MIX.exe
```

程序会请求管理员权限。左侧按钮用于切换完整功能页。最小化隐藏至系统托盘，单击/双击恢复窗口，右击打开托盘菜单。恢复时宿主重新布局、遮罩并重绘当前嵌入页，不重启组件或后台任务。

## 界面外观

主窗口、三个独立组件窗口和 MOD 冲突弹窗尝试启用系统支持的深色标题栏；效果取决于 Windows 版本和主题。左侧标签页按功能使用青色、绿色、紫色，当前页和按下状态使用更亮的强调色。

各功能页按钮也按动作类型统一上色：刷新、检测、浏览和扫描偏蓝；启动、注入、部署和确认偏绿；合并、推荐和分类处理偏紫；停止和取消偏红；恢复默认和还原偏橙。不可用按钮会自动变暗并使用灰色文字，弹出窗口中的确认、取消、推荐按钮也遵循同一套配色。

## 标签页切换与后台任务

左侧的 `连跳辅助`、`组服务器过滤` 和 `MOD 分类合并` 按钮是三个标签页，不是任务启动/停止开关。

切换标签时，程序按以下顺序处理：

1. 禁用未激活页面的根窗口，使该页面内的按钮、输入框和其他控件全部无法点击，也无法接收键盘输入。
2. 隐藏未激活页面及其页面容器。
3. 使用统一深色背景完整遮罩旧页面，避免控件残影或页面重叠。
4. 显示目标标签页面，并恢复该页面根窗口的交互能力。

切换标签只改变 UI 的可见性和交互状态，不会暂停、取消或重启后台任务：

- 连跳辅助已经启动时，切换到过滤器标签后，连跳工作 goroutine 仍会继续读取游戏状态并运行。
- 过滤器正在执行启动、注入、PowerShell 或日志相关任务时，切换到连跳标签后，相关任务和超时计时仍会继续。
- MOD 分类合并页正在扫描、合并、部署或还原时，切换到其他标签后，文件任务、进度计算和消息回传仍会继续。
- 未激活页面的消息循环、计时器和状态更新仍然运行；切回该标签后会显示最新状态。
- 重新激活页面只恢复页面根窗口，不会强制启用组件内部因“任务运行中”等原因自行禁用的按钮。
- MOD 冲突处理窗口在嵌入模式下属于 MOD 标签页面；切走时会一并遮罩和禁用，切回后恢复原冲突选择状态。
- MOD 冲突处理使用标签内部的全页模态遮罩层：遮罩覆盖主页面全部控件，手动处理窗口固定置于遮罩最上层。创建或置顶失败时会立即撤销遮罩并恢复主页面，不会留下整页不可点击的状态。

从系统托盘恢复窗口时，当前标签会执行一次比普通切换更强的刷新：宿主窗口先恢复显示和最大化，再隐藏当前页面宿主、清空内容区域、按最新客户区尺寸重显当前页面和嵌入子窗口。该流程只影响绘制与交互状态，不会重启子组件，也不会中断正在运行的扫描、注入或文件任务。

后台任务只会在以下情况下停止：

- 用户主动点击对应工具的停止按钮。
- 任务正常完成、自身超时或发生错误。
- 目标游戏进程关闭，任务按原工具逻辑退出。
- 用户关闭整个 `L4D2_MIX` 主程序。

维护此项目时，应保留“标签切换仅控制页面显示和输入，不控制后台任务生命周期”这一行为。

关闭主程序采用受控握手：

- MOD 扫描、合并、部署、还原或冲突处理仍在进行时，主程序拒绝退出，并自动切回 MOD 标签提示用户。
- 所有组件允许关闭后，主程序分别发送关闭消息，并等待三个组件进程实际结束后才销毁宿主窗口。
- 组件在正常关闭后 5 秒仍未退出时才执行超时清理，避免遗留失去父窗口的后台进程。

`MOD 分类合并` 的一键还原会恢复首次部署前的 `addonlist.txt` 基线，并移出当前合并包。还原成功后，工具会清理自己自动创建的 `addonlist.txt.l4d2modjoin.*.bak` 备份文件；如果还原被阻止或失败，这些备份会保留用于后续恢复。

首次运行时，单文件 EXE 会把内置运行组件释放到：

```text
%LOCALAPPDATA%\L4D2_MIX
```

内置的三个界面组件仍释放到该目录，但用户配置和日志统一保存在用户实际启动的 `L4D2_MIX.exe` 同级 `data` 目录：

```text
L4D2_MIX.exe
data\
├─ autobhop-settings.json
├─ row-filter\
│  ├─ blocked_keywords.txt
│  ├─ blocked_connectstrings.txt
│  ├─ learned_connectstrings.txt
│  ├─ auto_derived_connectstrings.txt
│  ├─ row_filter_mode.txt
│  └─ matchmaking_row_filter.log
├─ mod-join\
│  ├─ mod-scan-report.json
│  ├─ mod-conflict-policy.json
│  ├─ l4d2modjoin-build.json
│  ├─ .l4d2modjoin-deployment.json
│  └─ l4d2modjoin-settings.json
└─ matchmaking_probe_loader\
```

发布推荐使用 `dist\L4D2_MIX-portable.zip`，也可单独发布 `dist\L4D2_MIX.exe`。ZIP 只含 EXE 和 README.txt，旁边的 `.zip.sha256` 文件用于核对压缩包哈希。`dist\data` 包含用户规则、个人路径、部署记录和日志，不包含在便携包中。升级前备份并保留原 data。

EXE 首次运行会自动释放内置载荷并创建需要的 `data` 子目录。如果要做预设版便携包，可以手动放入整理干净的 `data\row-filter` 默认规则；`data\mod-join` 通常包含个人游戏路径、扫描报告和部署记录，不建议随公开版本发布。

`autobhop-settings.json` 保存连跳偏移、轮询间隔、探测来源和探测结果。旧文件名 `l4d2-autobhop-vpk-w.offsets.json` 带有融合前子项目名称，程序发现旧文件后会自动读取、迁移为新名称并删除旧文件。

过滤标签页没有额外生成一份重复 JSON。它实际使用四份可直接编辑的规则 TXT；注入 DLL、恢复默认脚本和管理器全部读写同一个 `data\row-filter`。`config_defaults\default_filter_config.json` 只是随组件提供的默认规则说明和种子快照，不是“保存配置”按钮的输出文件。

程序更新内置 DLL、Loader、脚本或默认模板时，不会覆盖 `data\row-filter` 中已经存在的可编辑规则、模式和日志。

### MOD 武器音效音量

`MOD 分类合并` 页面提供武器 VPK 专属音量设置：

- 下拉框固定档位：`0%`、`14%`、`42%`、`70%`、`100%`。
- 右侧 `自定义 %` 输入框可填写 `0` 到 `100` 的整数百分比。该输入框有内容时优先使用自定义值；留空时使用下拉框当前档位。
- 该设置保存在 `data\mod-join\l4d2modjoin-settings.json`，字段为 `weapon_sound_volume_percent`、`weapon_sound_volume_configured` 和 `custom_weapon_sound_volume`。

合并时，音量设置会挂到所有输出组，而实际写入 VPK 时只处理包内可识别的射击类武器 WAV：

- 命中范围默认为 `sound/weapons/*.wav`。
- 明确排除非射击武器或物品类目录：`melee`、`chainsaw`、`molotov`、`pipe_bomb`、`vomitjar`、`gascan`、`propanetank`、`oxygentank`、`fireworkcrate`、`cola_bottles`、`first_aid_kit`、`defibrillator`、`adrenaline`、`pain_pills`、`upgradepack`、`ammo_pack`。
- 因此纯音效 MOD 即使被动态分类到 `【07】Audio.vpk`，只要路径属于射击类武器音效，也会应用同一音量设置；`pistol`、`pump shotgun` 等不会因为不在 `Weapons` 输出组而漏处理。
- 智能扫描会检查每个射击类武器 MOD 是否包含 `sound/weapons` 音效。如果某个武器 MOD 只改模型/材质、没有自带音效，扫描日志会记录“武器音效待补入”和推断出的官方武器类型。
- 一键分类合并时，工具只针对扫描阶段标记的缺音效武器 MOD，从当前选择的游戏目录读取官方脚本（`scripts/weapon_*.txt` 与 `scripts/game_sounds*.txt`），按游戏自己的 `SoundData -> game_sounds -> wave` 引用链解析对应官方射击武器 WAV，再从官方 VPK 或官方 `sound\weapons` 散文件按原路径复制进 `Weapons` 输出包并一起应用音量设置。已自带 `sound/weapons` 音效的武器 MOD 不会触发官方音效补入。
- 官方脚本和资源按 L4D2 的 `gameinfo.txt` 搜索路径处理：`update` 优先于 `left4dead2_dlc3`、`left4dead2_dlc2`、`left4dead2_dlc1`，最后才是基础 `left4dead2`。同名 `game_sounds` 事件、同名 `weapon_*.txt` 脚本或同路径散文件存在多份时，工具使用更高优先级目录中的版本，避免拿到旧版官方音效。
- 官方资源来源包含 `update\pak01_dir.vpk`、`left4dead2_dlc*\pak01_dir.vpk`、`left4dead2\pak01_dir.vpk`，以及这些官方目录下的 `sound\weapons` 散文件。许多官方 WAV 并不在 VPK 内，而是以散文件存在；工具会用官方脚本解析出的路径去两类来源中定位实际文件。
- 因为官方音效补入需要读取游戏原始脚本和资源，使用该功能时 `游戏 Addons 目录` 必须指向真实安装目录下的 `Left 4 Dead 2\left4dead2\addons`。如果路径不正确、官方 `pak01_dir.vpk` 缺失，或官方脚本无法解析到对应射击武器 WAV，合并会停止并提示修正目录，而不是静默生成一个缺少官方音效副本的包。
- 官方音效副本使用官方脚本解析出的原始路径写入，例如 `sound/weapons/pistol/gunfire/pistol_fire.wav`。这样即使 MOD 或游戏脚本仍指向原音效路径，启用合并包后也会命中合并包内的降音量副本。
- 如果任一 MOD 已经提供同一路径的射击武器音效，工具不会用官方音效覆盖它，只会按当前音量设置处理 MOD 自带音效。
- 支持 PCM/float WAV 的采样缩放。缩放后会按最终内容重新计算 VPK 条目 CRC，保证后续构建清单校验和部署校验仍能通过。
- 非 WAV、压缩编码或无法识别的音频会保持原样，以避免破坏 VPK。

### 独立版 MOD 状态导入

首次启动融合版时，程序会检查原独立版目录：

```text
F:\Project\03_Game_Tools\L4D2_MOD_JOIN\dist\data
```

也可通过 `L4D2_MIX_LEGACY_MOD_JOIN_DATA` 指定其他来源。导入规则：

- 融合版目标文件不存在时，复制扫描报告、冲突策略、构建清单、部署记录和目录设置。
- 目标存在且内容相同时跳过。
- 目标存在且内容不同时不覆盖当前文件，旧文件保存在 `data\mod-join\legacy-import`。
- 导入结果记录在 `data\mod-join\legacy-import-v1.json`。
- 导入只复制文件，不删除或修改独立版目录。

## 图标

根目录的 `ico.jpg` 为 2048×2048 源图。构建使用 `assets\app.ico`，其中包含 16、20、24、32、40、48、64、128 和 256 像素图层。该图标被写入：

- `L4D2_MIX.exe` 文件资源
- 主窗口标题栏
- Windows 任务栏
- 系统托盘

## 构建

```powershell
.\package-portable.ps1
```

打包脚本会检查 `go` 和 `windres.exe` 是否在 `PATH` 中，并把 `GOCACHE`、`GOTMPDIR` 固定到项目内的 `.tmp` 目录，避免临时文件散落到系统目录。

构建需 Windows、Go 1.26（宿主及连跳模块要求）以及 PATH 中的 go 和 windres.exe。当前发布宿主为 x64，预构建 Loader/DLL 为 x86。脚本测试并重建三个 Go 界面组件，检查已有运行文件，生成载荷清单与图标资源，测试宿主并生成：

```text
dist\L4D2_MIX.exe
dist\L4D2_MIX-portable.zip
dist\L4D2_MIX-portable.zip.sha256
```

过滤器运行文件已纳入 `payload\runtime`，构建不访问其他项目源码。Loader/DLL 使用预构建文件，其源码和独立编译脚本未包含在本仓库。ZIP 是本地构建产物；Git 推送不发布 Release 附件。

构建脚本会检查每个原生命令的退出码。任一组件测试、组件构建、资源编译或宿主构建失败时会立即终止，不会继续使用旧载荷生成看似成功的 EXE。

需要顺手验证界面切换时，可以运行：

```powershell
.\package-portable.ps1 -VerifyUI
```

该参数在宿主构建后、ZIP 生成前启动新 EXE，执行 30 次标签循环并写入 `.tmp\switch-results.json`。需要处理 UAC 提示，脚本排除此前已存在的宿主进程。它仅验证切换；完整生命周期验证见下文。

## UI 防卡死约束

- 组件释放、进程启动和窗口等待均在 goroutine 中执行。
- 工作线程只用 `PostMessageW` 把结果交回 UI 线程。
- 宿主主消息循环不执行 PowerShell、不等待游戏、不扫描日志；过滤页自身仍读取规则和日志尾部，MOD 的迁移和路径检测在后台初始化。
- 三个原工具仍各自保留原有后台任务与超时机制。
- 三个功能组件在创建窗口时直接使用宿主页作为父窗口，不会先显示独立顶层窗口，也不再使用运行后的跨进程 `SetParent`。
- 页面切换隐藏的是原生子窗口页面；构建脚本附带 30 次三标签循环验证脚本 `scripts\verify-ui-switch.ps1`。
- 左侧三个入口按标签页处理：切换时先禁用并隐藏旧页面，以统一背景色完整遮罩，再显示并按原状态恢复新页面。未激活标签的所有控件均无法接收点击或键盘输入。
- 标签停用不发送 `WM_CLOSE`、停止通道或 Context 取消信号，因此后台 goroutine、子进程、计时器和消息循环不会因切换标签而中断。
- 托盘恢复会对当前标签执行强制隐藏、遮罩、重显和重绘；维护时不要删掉这一步，否则复杂 Win32 子页面在最小化恢复后可能出现控件残影。

## 风险提示

连跳页会只读游戏进程内存并模拟按键；过滤器页会把本项目携带的 DLL 注入本机 L4D2。请仅在你拥有和信任的本机环境中使用，并自行遵守服务器、平台和社区规则。


## 启动性能与诊断

- 默认连跳页先释放并创建进程；其他载荷准备与旧 MOD 状态导入和连跳初始化重叠，默认页初始化完成后再创建其他页面进程。其余页面自动启动，不是点击标签才加载。默认页优先可能使非当前页稍晚就绪。
- 页面完成初始化后设置 `L4D2MixReady`，宿主检查进程 ID、直接父窗口和就绪标记；接入并布局后设置 `L4D2MixAttached`。计时脚本同时检查两个标记。加载失败或进程退出显示明确错误。
- 构建自动生成 `payload/startup-manifest.json`。同一版本的固定组件在 24 小时内使用大小/修改时间缓存；文件缺失、大小/时间变化、版本变化、缓存损坏或到期时重新校验 SHA-256 并修复。可编辑规则不覆盖。
- 元数据缓存不是逐次完整性校验：同大小、同修改时间的改动在缓存期内可能延迟发现。需要每次完整校验时，设置 `L4D2_MIX_VERIFY_PAYLOAD=1`。
- MOD 页先创建控件，后台迁移状态并读取配置；保存过的路径直接使用，不在启动时重新验证可访问性。首次检测只探测本地固定磁盘；已有网络盘路径保留。初始化期间禁用相关操作，完成后恢复。
- 日常启动不写性能日志。设置 `L4D2_MIX_TRACE_STARTUP=1` 后，日志按 EXE 名追加至 EXE 同级 `data/startup-traces/*.jsonl`，记录 session、pid、phase、ms，覆盖初始化、接入和首次绘制。隐藏页可能首次显示时才记录 first_paint。计时脚本也计入进入 Go main 前的进程创建等待。

```powershell
# 构建、测试并生成不含个人 data 的便携 ZIP
.\package-portable.ps1

# 在独立数据/组件缓存中测量首次释放与后续启动
.\scripts\measure-startup.ps1 -Exe .\dist\L4D2_MIX.exe `
  -FixtureRoot .\.tmp\startup-benchmark -OutputPath .\.tmp\startup-results.json -Runs 6 -RequireReady

# 启动中关闭、页面切换、托盘恢复、子进程退出验证
.\scripts\verify-startup-lifecycle.ps1 -Exe .\dist\L4D2_MIX.exe `
  -FixtureRoot .\.tmp\lifecycle-fixture -OutputDir .\.tmp\lifecycle-results
```

首次释放组件缓存不等于操作系统冷启动。磁盘、系统负载及进入程序前的 EXE 扫描会影响测量；不要从一次测量推断固定加速比例。


### 首帧闪白修复

宿主的侧栏、内容区和页面容器使用专门的深色窗口类，不再依赖嵌套 `STATIC` 控件的系统默认背景。宿主处理 `WM_PAINT`/`WM_ERASEBKGND`，创建时即使用最大化布局；子页面先调整尺寸，再显示并同步完成首次重绘。MOD 窗口类也使用真实深色背景刷，避免首帧露出系统白色背景。

## 测试、验证边界与问题定位

先执行打包脚本生成 Go 界面组件和载荷清单，再测试。宿主和三个组件是独立 Go 模块；宿主的 `go test ./...` 不会自动测试嵌套模块。

```powershell
foreach ($module in @('.', 'components/autobhop', 'components/rowfilter', 'components/modjoin')) {
    Push-Location $module
    try {
        go test -race ./...
        if ($LASTEXITCODE -ne 0) { throw "Tests failed: $module" }
        go vet -unsafeptr=false ./...
        if ($LASTEXITCODE -ne 0) { throw "Static checks failed: $module" }
    } finally { Pop-Location }
}

# 宿主的深色背景像素回归
go test -run TestDarkPanelPaintsDarkInsteadOfSystemWhite -v .

# 与 .zip.sha256 文件第一列核对
Get-FileHash .\dist\L4D2_MIX-portable.zip -Algorithm SHA256
```

完整 `go vet` 在四模块合计报告 6 处既有 Win32 回调 lParam 转原生指针提示（宿主 2、连跳 1、过滤器 1、MOD 2）。上面的静态检查明确排除该项，不代表完整 vet 零提示；竞态检测需要支持的平台与 C 工具链。

生命周期脚本使用独立 FixtureRoot，验证六种启动中关闭时机、遗留子进程、30 次标签切换、托盘恢复，并有意终止自己的过滤器测试进程。它会启动管理员窗口，旧 MOD 状态仍可能按程序规则复制到测试目录。不要把 FixtureRoot 指向正在使用的 data 或缓存。`-VerifyUI` 仅验证标签切换，不替代完整生命周期脚本。

启用诊断时，在启动 EXE 的同一 PowerShell 会话中设置 `$env:L4D2_MIX_TRACE_STARTUP = '1'`；结束后清除该变量或关闭该会话。开关只影响后续创建的进程。日志按 EXE 名追加，反馈前区分 session；分享日志时移除个人路径与配置。

首次释放组件缓存不等于系统冷启动。当前验证没有采样每个桌面合成帧；若仍观察到闪白，先确认运行优化版新 EXE，再记录白色区域（整窗、内容区、标题栏）、Windows 版本、录屏和启动日志。不要把完整个人 data 随公开反馈上传。

升级回退应用时使用保留的旧 EXE 或源码版本。MOD 页的“一键还原”只还原游戏部署，不回滚应用程序版本。

## 全量后台审查与验证（2026-10-08）

```powershell
.\scripts\verify-background.ps1 -Build -Rounds 2
```

根目录执行时覆盖根目录与优化副本的四个 Go 模块，检查软件副本一致性、普通测试、重复竞态测试、完整 vet 已审查 ABI 提示、其他静态分析、gofmt，以及 ZIP 条目与 SHA-256。命令、工作目录、原始 stdout/stderr、退出码及产物哈希记录在 `.tmp/background-verification.json`。在副本执行时仅检查副本。失败立即终止并写出失败记录。

默认不启动应用、游戏、注入脚本或可见 UI 验证；原生窗口测试只创建不可见 HWND 和离屏位图。不要传 `package-portable.ps1 -VerifyUI`，也不要调用其他 UI/启动测量脚本来进行纯后台验证。真实首帧合成效果和游戏集成需要另行实机验证。

本轮修复包括：关闭检查采用有界跨进程消息（超时保持任务运行）、激活通知异步投递、接入前重新核对 HWND/PID/父窗口/Ready、连跳工作线程退出后再释放所有权、重复枚举复用原生回调、日志尾读按快照长度限流、VPK 读取先校验范围、合并第二遍读取核对长度/CRC，以及所有计划路径预检以防输出越界或覆盖输入。构建脚本恢复调用方 GOCACHE/GOTMPDIR，并禁用测试缓存。

“后台审查无新增可修复问题”不等于对所有机器、输入和桌面帧的绝对无缺陷保证；Loader/DLL 为预构建组件，本仓库没有它们的源码。
