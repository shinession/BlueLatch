# English

## ADR 0006: The icon is Snap on a dark tile, embedded through a resource file that `tools/mkrsrc` writes

Status: accepted on 2026-11-24, by the user. Built in version 0.1.2.

### Context

The programs needed an icon: for the file in Explorer and in shortcuts, for the title bar and the taskbar, and for the README.

Seven candidates were drawn and compared at 256 to 16 px, on light and dark backgrounds, and at true pixels: A Link (two woven chain links), B Monogram (a B), C Snap (a diamond cut into two halves that a latch tongue snaps together) in three variants (C1 on a light tile, C2 on a dark tile, C3 without a tile), D Earbuds, and E Ping (rings with a node). All of them avoid the Bluetooth rune, which is a trademark of the Bluetooth SIG.

### Decision

Use C2. Snap is the only candidate that draws "latch", and its silhouette is not shared with common symbols. The dark tile keeps its edge on light and on dark taskbars.

- The source is `assets/logo/bluelatch.svg`. `scripts\logo-frames.ps1` renders the PNG frames in `assets/logo/png` from it: 16, 20, 24, 32, 40, 48, 64, 128, 256, and 512 for the README. The icon of the programs has the frames up to 256.
- The icon and the version information are put into the programs through a `.syso` file in the directory of each main package, which the Go toolchain links by itself. `tools/mkrsrc` writes these files with the standard library only, and `scripts\resources.ps1` runs it for both programs and both architectures.

Options for the embedding:

1. `go-winres` or `rsrc`: third-party tools that would have to be downloaded and run. Not taken: the need is small, an icon and a version.
2. Writing the resources into the finished executable with `UpdateResource`: every build would need that extra step, and a plain `go build` would give a program without an icon.
3. Setting the icon at run time only: Explorer, shortcuts, and a pinned taskbar button need the icon in the file.

### Consequences

- Both programs have the icon and the version information (the Details tab of the properties), for amd64 and arm64. Four `.syso` files of about 170 KB each are committed, so building needs no tool.
- When the icon or the version changes, run `scripts\resources.ps1 -Version <version>` and commit the files. A change of the SVG needs `scripts\logo-frames.ps1` first.
- The window sets the icons of its title bar and its taskbar button at the DPI of the window (`setWindowIcons`), from the same resource. A program built without the resources keeps the default icon.
- There is no manifest. The window sets its DPI awareness at run time. A wrong manifest keeps a program from starting, and nothing here needs one.
- Verified: both programs link with the files, `FileVersionInfo` reads the version information, and Windows extracts the icon at 16, 24, 32, 48, and 256 px. The icon shows in the title bar of the window. Not looked at: Explorer's large-icon views and a pinned taskbar button.
- Since version 0.1.3 there is one program (ADR 0008), so two `.syso` files are committed (`cmd/bluelatch/rsrc_windows_*.syso`), not four, and `scripts\resources.ps1` writes those.

---

# 中文

## ADR 0006：图标是深色底的 Snap，通过 `tools/mkrsrc` 生成的资源文件嵌入

状态：2026-11-24 由用户接受。已在 0.1.2 版中构建。

### 背景

程序需要一个图标：用于资源管理器和快捷方式里的文件、标题栏和任务栏，以及 README。

我们画了七个备选，并在 256 到 16 像素、浅色和深色背景下，以及真实像素下做了比较：A Link（两个织在一起的链环）、B Monogram（字母 B）、C Snap（一颗菱形被锁舌咬合成两半）及其三个变体（C1 浅色底、C2 深色底、C3 无底板）、D Earbuds（耳塞）、E Ping（带节点的同心环）。所有备选都避开了蓝牙符文，它是 Bluetooth SIG 的商标。

### 决定

采用 C2。Snap 是唯一把"latch"画出来的备选，轮廓也不和常见符号重复。深色底板让图标在浅色和深色任务栏上都保持边缘清晰。

- 源文件是 `assets/logo/bluelatch.svg`。`scripts\logo-frames.ps1` 从它渲染出 `assets/logo/png` 中的 PNG 帧：16、20、24、32、40、48、64、128、256，以及用于 README 的 512。程序的图标包含不超过 256 的帧。
- 图标和版本信息通过放在每个 main 包目录中的 `.syso` 文件放进程序，Go 工具链会自动链接它。`tools/mkrsrc` 只用标准库写出这些文件，`scripts\resources.ps1` 为两个程序和两种架构运行它。

嵌入方式的备选：

1. `go-winres` 或 `rsrc`：第三方工具，需要下载并运行。不采用：需求很小，只是一个图标和一个版本。
2. 用 `UpdateResource` 把资源写进已生成的可执行文件：每次构建都要多做这一步，直接 `go build` 得到的程序会没有图标。
3. 只在运行时设置图标：资源管理器、快捷方式和固定在任务栏的按钮都需要文件里有图标。

### 后果

- 两个程序都有图标和版本信息（属性的"详细信息"页），支持 amd64 和 arm64。提交了四个各约 170 KB 的 `.syso` 文件，所以构建不需要任何工具。
- 图标或版本变化时，运行 `scripts\resources.ps1 -Version <version>` 并提交这些文件。SVG 有变化时，要先运行 `scripts\logo-frames.ps1`。
- 窗口用同一份资源，按窗口所在的 DPI 设置标题栏和任务栏按钮的图标（`setWindowIcons`）。没有资源构建出的程序保持默认图标。
- 没有 manifest。窗口在运行时设置 DPI 感知。错误的 manifest 会让程序无法启动，而这里没有任何东西需要它。
- 已验证：两个程序都能与这些文件链接，`FileVersionInfo` 能读出版本信息，Windows 能在 16、24、32、48 和 256 像素下取出图标。图标显示在窗口的标题栏上。没有看过：资源管理器的大图标视图和固定在任务栏的按钮。
- 从 0.1.3 版起只有一个程序（ADR 0008），所以提交的是两个 `.syso` 文件（`cmd/bluelatch/rsrc_windows_*.syso`），而不是四个，`scripts\resources.ps1` 写出的就是它们。
