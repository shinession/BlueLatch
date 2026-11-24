# English

## ADR 0003: The window is a WebView2 window with embedded HTML

Status: accepted on 2026-11-24, by the user. Built in version 0.1.1. The binding was chosen when the window was built (see Decision).

### Context

The program needs a window with the paired devices, a **Connect** button for each, and the progress of the retries. The development PC has no C compiler (cgo is off) and no Node. Windows Settings on it is in dark mode.

Options:

1. A WebView2 window showing HTML, CSS, and JavaScript embedded in the executable, through a Go binding that needs no cgo. The page follows the dark or light theme of the system with the CSS `prefers-color-scheme`, it is easy to change and to check with screenshots, and the executable stays small. It needs the WebView2 runtime, which is installed on the development PC (version 157.0.4341.60) and is part of Windows 11.
2. Native Win32 controls (walk). No runtime dependency, but as far as we know no dark mode: a bright window on a dark desktop.
3. A tray icon with a menu. The lightest, but it cannot show the progress of the retries.
4. Fyne needs a C compiler on Windows, and Wails needs Node for its front end. Neither is possible on this PC without installing more.

### Decision

Use option 1. The retry engine and `internal/bt` stay independent of the window, so the window can be replaced.

The binding is `github.com/jchv/go-webview2` at `v0.0.0-20260205173254-56598839c808` (MIT licence, last commit 2026-02-05). It needs no cgo, embeds `WebView2Loader.dll` and loads it from memory (through `github.com/jchv/go-winloader`), and creates the window, so BlueLatch has little Win32 code of its own. The alternative looked at was `github.com/wailsapp/go-webview2` v1.0.23 (the one under Wails), which is lower level: the window and its message loop would be BlueLatch's own code. It remains the fallback if the chosen binding gets into trouble.

### Consequences

- The WebView2 runtime must be present. Windows 11 has it. Windows 10 may need it installed. When it is missing, the window program shows a message box that names the download page.
- A WebView2 window uses more memory than native controls, so it is not meant to stay resident in the tray.
- The page reads the state of the application every 500 ms (the bound function `blState`) instead of being pushed to. As far as the code of the binding shows, a push from another thread is a message posted to the thread of the window, and such messages are not delivered while the user drags the window. A state that is read again cannot get stale.
- The binding closes the window when it gets `WM_CLOSE`, so the window procedure is wrapped to delay that until a request that Windows is working on has finished ([ADR 0005](./0005-journal-of-service-switches.md)). The binding also does not set the DPI awareness or the dark title bar; BlueLatch does.
- Device names are set in the page as text, never as HTML (AGENTS.md section 7), and the page carries a content security policy that allows nothing from outside.
- The binding does not give access to every WebView2 setting, so the page itself blocks reload, print, zoom, and dropped files.
- The console subsystem question is settled in [ADR 0004](./0004-the-window-is-a-separate-executable.md).

---

# 中文

## ADR 0003：窗口是内嵌 HTML 的 WebView2 窗口

状态：2026-11-24 由用户接受。已在 0.1.1 版中构建。绑定是在构建窗口时选定的（见"决定"）。

### 背景

程序需要一个窗口，显示已配对的设备，每个设备一个 **Connect** 按钮，并显示重试的进度。开发用电脑没有 C 编译器（cgo 关闭），也没有 Node。电脑上的 Windows 设置是深色模式。

备选方案：

1. WebView2 窗口，显示嵌入在可执行文件中的 HTML、CSS 和 JavaScript，通过不依赖 cgo 的 Go 绑定。页面用 CSS `prefers-color-scheme` 跟随系统的深色或浅色主题，容易修改，也容易用截图检查，可执行文件保持较小。它需要 WebView2 运行时，开发用电脑上已安装（版本 157.0.4341.60），并且它是 Windows 11 的一部分。
2. 原生 Win32 控件（walk）。没有运行时依赖，但据我们所知不支持深色模式：在深色桌面上是一个明亮的窗口。
3. 带菜单的托盘图标。最轻量，但无法显示重试的进度。
4. Fyne 在 Windows 上需要 C 编译器，Wails 的前端需要 Node。在这台电脑上，不再安装其他东西，两者都无法使用。

### 决定

采用方案 1。重试引擎和 `internal/bt` 保持独立于窗口，所以窗口可以被替换。

绑定是 `github.com/jchv/go-webview2`，版本 `v0.0.0-20260205173254-56598839c808`（MIT 许可，最后一次提交于 2026-02-05）。它不需要 cgo，内嵌了 `WebView2Loader.dll` 并从内存加载（通过 `github.com/jchv/go-winloader`），并且负责创建窗口，所以 BlueLatch 自己的 Win32 代码很少。曾经比较过的另一个是 `github.com/wailsapp/go-webview2` v1.0.23（Wails 底层使用的那个），它层次更低：窗口和它的消息循环需要由 BlueLatch 自己编写。如果所选的绑定出了问题，它仍然是后备方案。

### 后果

- 必须有 WebView2 运行时。Windows 11 自带，Windows 10 可能需要安装。缺少时，窗口程序会显示一个消息框，其中给出下载页面。
- WebView2 窗口比原生控件占用更多内存，所以不适合常驻托盘。
- 页面每 500 毫秒读取一次应用的状态（绑定函数 `blState`），而不是被推送。据绑定的代码来看，从其他线程推送是向窗口所在线程投递一条消息，而这类消息在用户拖动窗口期间不会被送达。重新读取的状态不会过时。
- 绑定收到 `WM_CLOSE` 时会直接关闭窗口，所以窗口过程被包装起来，把这件事推迟到 Windows 正在处理的请求结束之后（[ADR 0005](./0005-journal-of-service-switches.md)）。绑定也不设置 DPI 感知和深色标题栏，这些由 BlueLatch 设置。
- 设备名称在页面中以文本方式设置，绝不作为 HTML（AGENTS.md 第 7 节），页面还带有不允许加载任何外部内容的内容安全策略。
- 绑定没有开放 WebView2 的所有设置，所以由页面自己阻止刷新、打印、缩放和拖入文件。
- 控制台子系统的问题在 [ADR 0004](./0004-the-window-is-a-separate-executable.md) 中解决。
