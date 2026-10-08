# L4D2 MIX

[GitHub 仓库](https://github.com/STC214/L4D2_MIX) · [优化版完整说明](L4D2_MIX_optimized/README.md)

文档更新：2026-10-08（Asia/Shanghai）。推荐使用优化版；根目录原版保留原始实现，未同步启动优化、组件缓存和首帧闪白修复。

| 实现 | 启动路径（相对仓库根目录） | 构建入口 |
| --- | --- | --- |
| 优化版（推荐） | `L4D2_MIX_optimized/dist/L4D2_MIX.exe` | 进入 `L4D2_MIX_optimized` 后运行 `package-portable.ps1` |
| 根目录原版 | `dist/L4D2_MIX.exe` | 在仓库根目录运行 `package-portable.ps1` |

两版 EXE 文件名相同，请核对实际运行路径。下面的功能与数据说明适用于融合工具；涉及启动优化与 ZIP 的部分仅适用于优化版。

一个统一的 Go + Win32 控制台，将以下三个现有工具放进同一个窗口：

- `Left4Dead2-Autobhop_VPK_W`：窗口选择、偏移配置、自动探测、启动/停止和实时状态。
- `L4D2RowFilterManager.exe`：规则编辑、恢复默认、启动并注入、注入运行中的游戏、日志与组件目录。
- `L4D2_MOD_JOIN`：VPK 动态扫描分类、冲突处理、分类合并、武器音效音量处理、安全部署和还原。分类输出包使用 `【01】UI_HUD.vpk`、`【02】Survivors.vpk` 这样的括号编号格式。

## 使用

推荐运行优化版：

```text
L4D2_MIX_optimized\dist\L4D2_MIX.exe
```

程序会请求管理员权限。左侧按钮用于切换三个完整功能页。最小化隐藏至系统托盘：优化版单击/双击恢复，右击打开托盘菜单；原版单击、双击或右击恢复。恢复时宿主重新布局并重绘当前嵌入窗口，不重启组件或后台任务。

## 优化版子项目

当前仓库保留母项目原始实现，优化版位于 `L4D2_MIX_optimized`。它保持功能入口和运行数据布局，并维护 UI 外观、启动调度、固定组件缓存、首帧深色绘制、诊断和便携 ZIP。继续迭代优先进入该子项目：

```powershell
cd .\L4D2_MIX_optimized
.\package-portable.ps1
```

优化版主窗口、独立组件窗口和 MOD 冲突处理弹窗会启用深色标题栏；左侧标签页按功能使用青色、绿色、紫色区分；各功能页和弹窗按钮按动作类型统一配色，例如启动、注入、部署和确认偏绿，刷新、检测、浏览和扫描偏蓝，停止和取消偏红，恢复默认和还原偏橙。

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

发布给别人使用时，通常只需要发布 `dist\L4D2_MIX.exe`。`dist\data` 是本机运行和验证时生成的状态目录，包含用户规则、扫描缓存、部署记录、日志和窗口设置，公开发布时一般不要一起带上。

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

## 优化版启动、发布与文档导航

- 默认连跳组件先校验/释放并创建进程；其他载荷准备与旧 MOD 状态导入和连跳初始化重叠，默认页初始化完成后再创建其他页面进程。默认页优先可能让非当前页稍晚就绪，不保证所有页面同时变快。
- 子页面设置 `L4D2MixReady`，宿主检查进程 ID、直接父窗口和该标记；接入并布局后设置 `L4D2MixAttached`。加载和进程退出显示明确状态。
- 固定组件按构建清单缓存：24 小时内大小/修改时间一致时跳过重复读取；版本、元数据变化、缺失、缓存损坏或过期触发 SHA-256 校验与修复。相同大小和修改时间的变动可能延迟发现，设置 `L4D2_MIX_VERIFY_PAYLOAD=1` 可每次完整校验。已有可编辑规则、模式、日志保留。
- 宿主采用专用深色容器与显式背景绘制，创建时采用最大化布局；子页先调整尺寸，再显示并同步首次重绘，MOD 窗口类也使用深色背景刷。
- 设置 `L4D2_MIX_TRACE_STARTUP=1` 后，启动阶段日志追加至 EXE 同级 `data/startup-traces/*.jsonl`；平时不写性能日志。
- 优化版构建输出 `dist/L4D2_MIX.exe`、`dist/L4D2_MIX-portable.zip` 和 `.zip.sha256`。ZIP 只含 EXE 和 README.txt，不含个人 data。产物不由 Git 跟踪，推送源码不等于发布 Release 附件。
- 当前发布宿主为 x64，预构建 Loader/DLL 为 x86。宿主与连跳模块要求 Go 1.26；Windows 构建需将 go 和 windres.exe 放入 PATH。

[优化版使用与验证](L4D2_MIX_optimized/README.md) · [优化版 Loader](L4D2_MIX_optimized/payload/runtime/matchmaking_probe_loader/README.md) · [优化版过滤器](L4D2_MIX_optimized/payload/runtime/matchmaking_row_filter_dll/README.md) · [原版 Loader](payload/runtime/matchmaking_probe_loader/README.md) · [原版过滤器](payload/runtime/matchmaking_row_filter_dll/README.md)

首次释放组件缓存不等于操作系统冷启动。验证覆盖被测构建的深色背景像素、缓存、竞态、标签切换、启动中关闭和托盘恢复，不保证任意机器的固定启动时间或每个桌面合成帧。完整 go vet 仍有两处既有 Win32 lParam 指针转换提示；当前静态验证使用 `go vet -unsafeptr=false ./...`，不代表完整 vet 零提示。

## 图标资源

根目录的 `ico.jpg` 为 2048×2048 源图。构建使用 `assets\app.ico`，其中包含 16、20、24、32、40、48、64、128 和 256 像素图层。该图标被写入：

- `L4D2_MIX.exe` 文件资源
- 主窗口标题栏
- Windows 任务栏
- 系统托盘

## 构建

此处为**根目录原版**构建。推荐优化版的构建及 ZIP 流程见上文链接；运行前确认所在目录。

```powershell
.\package-portable.ps1
```

脚本测试并重建三个 Go 界面组件，检查仓库中已有运行文件、生成资源并构建：

```text
dist\L4D2_MIX.exe
```

过滤器 Loader、DLL、脚本和默认配置已纳入本项目的 `payload\runtime`。构建脚本不会再访问其他项目的绝对路径，因此当前项目可以独立构建。

Loader/DLL 使用预构建文件，其独立源码和编译脚本未包含在本仓库。原版脚本仅生成其 EXE，没有优化版 ZIP、缓存和首帧修复。升级前备份并保留 data；公开分发排除个人路径、规则和部署记录。MOD 的一键还原不回滚应用版本。

构建脚本会检查每个原生命令的退出码。任一组件测试、组件构建、资源编译或宿主构建失败时会立即终止，不会继续使用旧载荷生成看似成功的 EXE。

## UI 防卡死约束

- 组件释放、进程启动和窗口等待均在 goroutine 中执行。
- 工作线程只用 `PostMessageW` 把结果交回 UI 线程。
- 宿主主消息循环不执行 PowerShell、不等待游戏、不扫描日志；过滤器自身仍读取规则和日志尾部，优化版 MOD 的迁移与路径检测在后台初始化。
- 三个原工具仍各自保留原有后台任务与超时机制。
- 三个功能组件在创建窗口时直接使用宿主页作为父窗口，不会先显示独立顶层窗口，也不再使用运行后的跨进程 `SetParent`。
- 页面切换隐藏的是原生子窗口页面；构建脚本附带 30 次三标签循环验证脚本 `scripts\verify-ui-switch.ps1`。
- 左侧三个入口按标签页处理：切换时先禁用并隐藏旧页面，以统一背景色完整遮罩，再显示并按原状态恢复新页面。未激活标签的所有控件均无法接收点击或键盘输入。
- 标签停用不发送 `WM_CLOSE`、停止通道或 Context 取消信号，因此后台 goroutine、子进程、计时器和消息循环不会因切换标签而中断。
- 托盘恢复会对当前标签执行强制隐藏、遮罩、重显和重绘；维护时不要删掉这一步，否则复杂 Win32 子页面在最小化恢复后可能出现控件残影。

## 风险提示

连跳页会只读游戏进程内存并模拟按键；过滤器页会把本项目携带的 DLL 注入本机 L4D2。请仅在你拥有和信任的本机环境中使用，并自行遵守服务器、平台和社区规则。
