# English

## ADR 0008: One executable: a console program that frees its console when it opens the window

Status: accepted on 2026-11-24, by the user, who asked for one release file and took the recommendation below. Built in version 0.1.3. It supersedes [ADR 0004](./0004-the-window-is-a-separate-executable.md).

### Context

ADR 0004 chose two executables, `bluelatch.exe` for the console and `bluelatch-gui.exe` for the GUI subsystem. It rejected the two ways to have one file with "as far as we know". The user now wants the command line and the window in one release file. What ADR 0004 assumed was measured on 2026-11-24 on the development PC (Windows 11 Enterprise LTSC 10.0.26100, Windows Terminal 1.25 installed, the default terminal left at "let Windows decide"), with small probe programs that are not part of the repository:

- A program for the GUI subsystem that runs for 2 s: PowerShell returns after 0.02 s and `$LASTEXITCODE` stays empty. The same program for the console subsystem: PowerShell waits the 2 s and sees the exit code 7. `cmd.exe /c` waits for both and sees the exit code 7. The interactive prompt of `cmd.exe` was not tried.
- A program for the console subsystem that Explorer starts is alone on its console: `GetConsoleProcessList` returns 1. Started by `cmd.exe` it returns 2, and started from the PowerShell of the tool, 3.
- With `FreeConsole` as the first thing done, the window of Windows Terminal that the double click opened was visible for 31, 28, and 25 ms in three runs of a probe, and no window was left while the program went on running. Without it, the window stays for as long as the program runs.

Options:

1. One console program that frees its console when it opens the window.
2. One GUI program that attaches to the console of its parent when it is started with a command. PowerShell does not wait for it and does not see its exit code (measured), and, as far as we know, the prompt of `cmd.exe` does not wait either.
3. Two programs, as in ADR 0004.

### Decision

Use option 1. The command line keeps its contract, because a shell waits for the program and sees its exit code (AGENTS.md section 6). A double click costs a console window for about 30 ms.

- `bluelatch` without a command, and `bluelatch window [flags]`, open the window. `bluelatch help` prints the usage. The flags of `bluelatch-gui` (`-lang`, `-v`, `-debug`, `-demo`, `-demo-connect`, `-demo-disconnect`) are the flags of `bluelatch window`.
- How the window opens depends on how the process is attached to a console, which `GetConsoleProcessList` tells:
  - Alone on it (a double click, a shortcut, the Start menu): `FreeConsole` is the first thing the program does, and the window opens in this process.
  - Shared with other processes (a terminal): the flags are checked, with a mistake on stderr and exit code 2. Then the program starts itself again as `bluelatch window ...` without a console (`DETACHED_PROCESS`) and returns 0, so the prompt comes back at once. That process has `BLUELATCH_DETACHED=1` in its environment, which makes it count as having no console, so it can never start another one.
  - No console: the window opens in this process.
- The process of the window has no console, so its problems are shown in a message box and written to the log file.
- `list`, `connect`, `disconnect`, and `version` are as before and never touch the console.

### Observed

On 2026-11-24, with this program on the development PC: a double click on `bluelatch.exe`, and a shortcut to it with the arguments `window -demo`, each showed one window of Windows Terminal for about 30 ms (30 ms and 31 ms, first seen about 350 ms after the launch), and then only the window of BlueLatch. Typed in a terminal, `bluelatch window -demo` returned after 44 ms with exit code 0 and the window of BlueLatch was the only new window; a second start returned after 53 ms and left one window open. Through `cmd.exe`, `bluelatch disconnect` without a name gave the errorlevel 2.

Not measured: a PC without Windows Terminal (the classic console host), Windows 10, and the interactive prompt of `cmd.exe`.

### Consequences

- `release\` holds one file, `bluelatch.exe` (4.1 MB; the two programs of 0.1.2 were 3.5 MB and 3.9 MB). `bluelatch-gui.exe` is gone, so shortcuts to it have to point to `bluelatch.exe`. `cmd/bluelatch-gui` and its resource files are gone; two `.syso` files remain.
- A double click shows a console window for about 30 ms. Windows creates it before the program can do anything about it. If that turns out to bother, option 2 is the way back, with the drawbacks above.
- `bluelatch` without arguments no longer prints the usage and exits with 2: it opens the window, and the usage is `bluelatch help`. This is the one change to the contract of the command line.
- Opening the window from a terminal starts a second process, and the first returns 0 even when the window then fails. The problem is shown in a message box and written to the log file.
- Do not build the program with `-H=windowsgui`: a shell would stop waiting for it, and the command line would lose its exit codes.

---

# 中文

## ADR 0008：一个可执行文件：打开窗口时释放自己控制台的控制台程序

状态：2026-11-24 由用户接受，用户要求只发布一个文件，并采纳了下面的推荐。已在 0.1.3 版中构建。它取代 [ADR 0004](./0004-the-window-is-a-separate-executable.md)。

### 背景

ADR 0004 选择了两个可执行文件：控制台用的 `bluelatch.exe` 和 GUI 子系统用的 `bluelatch-gui.exe`。它用"据我们所知"否决了做成一个文件的两种办法。现在用户希望命令行和窗口放在同一个发布文件里。ADR 0004 所假定的内容，已于 2026-11-24 在开发用电脑上（Windows 11 Enterprise LTSC 10.0.26100，装有 Windows Terminal 1.25，默认终端保持"由 Windows 决定"）用不属于仓库的小探测程序测量过：

- 一个运行 2 秒的 GUI 子系统程序：PowerShell 在 0.02 秒后就返回，`$LASTEXITCODE` 仍为空。同一个程序按控制台子系统构建：PowerShell 等待这 2 秒，并拿到退出码 7。`cmd.exe /c` 对两者都会等待，并拿到退出码 7。`cmd.exe` 的交互式提示符没有试过。
- 由 Explorer 启动的控制台子系统程序独占它的控制台：`GetConsoleProcessList` 返回 1。由 `cmd.exe` 启动时返回 2，从工具的 PowerShell 启动时返回 3。
- 把 `FreeConsole` 作为第一件事时，双击所打开的 Windows Terminal 窗口，在探测程序跑的三次中分别可见 31、28 和 25 毫秒，并且程序继续运行期间没有留下任何窗口。不这样做的话，窗口会一直留到程序结束。

备选方案：

1. 一个控制台程序，打开窗口时释放自己的控制台。
2. 一个 GUI 程序，带命令启动时附加到父进程的控制台。PowerShell 不会等待它，也拿不到它的退出码（已测量），而且据我们所知，`cmd.exe` 的提示符同样不会等待。
3. 两个程序，如 ADR 0004。

### 决定

采用方案 1。命令行保持原有约定，因为 shell 会等待程序并拿到它的退出码（AGENTS.md 第 6 节）。代价是双击时会有一个控制台窗口出现约 30 毫秒。

- 不带命令的 `bluelatch` 和 `bluelatch window [flags]` 打开窗口。`bluelatch help` 打印用法。`bluelatch-gui` 的参数（`-lang`、`-v`、`-debug`、`-demo`、`-demo-connect`、`-demo-disconnect`）就是 `bluelatch window` 的参数。
- 窗口怎么打开，取决于进程是如何附着在控制台上的，这由 `GetConsoleProcessList` 告知：
  - 独占控制台（双击、快捷方式、开始菜单）：程序做的第一件事是 `FreeConsole`，窗口在本进程中打开。
  - 与其他进程共享（终端）：先检查参数，有错就写到 stderr 并返回退出码 2。然后程序以 `bluelatch window ...` 再启动自己一次，不带控制台（`DETACHED_PROCESS`），并返回 0，所以提示符立即回来。那个进程的环境里带有 `BLUELATCH_DETACHED=1`，这让它按没有控制台处理，所以它绝不会再启动另一个。
  - 没有控制台：窗口在本进程中打开。
- 窗口的进程没有控制台，所以它的问题显示在消息框里，并写入日志文件。
- `list`、`connect`、`disconnect` 和 `version` 与以前一样，从不触碰控制台。

### 观察到的情况

2026-11-24 在开发用电脑上用本程序：双击 `bluelatch.exe`，以及带参数 `window -demo` 的快捷方式，各自出现了一个 Windows Terminal 窗口，约 30 毫秒（30 毫秒和 31 毫秒，在启动后约 350 毫秒首次看到），之后只剩 BlueLatch 的窗口。在终端里输入 `bluelatch window -demo`，44 毫秒后返回，退出码 0，BlueLatch 的窗口是唯一新出现的窗口；第二次启动 53 毫秒后返回，仍只有一个窗口。通过 `cmd.exe` 运行不带名称的 `bluelatch disconnect`，errorlevel 为 2。

没有测量过：没有 Windows Terminal 的电脑（经典控制台宿主）、Windows 10，以及 `cmd.exe` 的交互式提示符。

### 后果

- `release\` 中只有一个文件 `bluelatch.exe`（4.1 MB；0.1.2 的两个程序分别是 3.5 MB 和 3.9 MB）。`bluelatch-gui.exe` 不再有，所以指向它的快捷方式要改为指向 `bluelatch.exe`。`cmd/bluelatch-gui` 及其资源文件已删除；剩下两个 `.syso` 文件。
- 双击时会有一个控制台窗口显示约 30 毫秒。Windows 在程序能做任何事之前就创建了它。如果这被证明令人困扰，方案 2 是退路，代价是上面所说的缺点。
- 不带参数的 `bluelatch` 不再打印用法并返回 2：它打开窗口，用法改用 `bluelatch help`。这是对命令行约定的唯一改动。
- 从终端打开窗口会启动第二个进程，而第一个进程即使在窗口随后失败时也返回 0。问题显示在消息框里，并写入日志文件。
- 不要用 `-H=windowsgui` 构建这个程序：shell 将不再等待它，命令行会失去退出码。
