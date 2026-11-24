# English

## ADR 0005: A journal lets a later run switch on a service that was left switched off

Status: accepted on 2026-11-24. Built in version 0.1.1.

### Context

`Reconnect` switches a service off and on again (ADR 0001). The log of a real run on 2026-11-24 (a speaker) shows what that means:

- Switching the A2DP Sink off took about 3.1 s, and the service was gone from the list of installed services while it was off. Switching it on took about 3.3 s, and the service was back in the list when the call returned.
- So for the whole of a request, about 6.5 s, the service is being switched off or is off, and a window that is closed or a process that is killed in that time may leave the service switched off. The service is then missing from the list, and `Reconnect` no longer sees it: when it was the only audio service of the device, it reports that the device has no audio service.

The window makes this much more likely than the command line.

Options:

1. Do nothing, and tell the user to pair the device again after a bad stop.
2. A journal: write a small file before a service is switched off, delete it once the service has been switched on again, and let the next start switch on whatever a file still describes.
3. Run each request in a helper process that is not stopped with its parent. It is more machinery, and it does not help when the helper is killed.
4. Do not switch off at all and only switch on. It is not known whether that connects a device, so it cannot be chosen without trying it on a device.

### Decision

Use option 2, together with a guard in the window: closing the window first stops the application and waits for a request that Windows is working on, at most 30 seconds (`app.Shutdown`).

- The journal is the folder `%LocalAppData%\BlueLatch\pending`. Each switch is one file, `pending-<address>-<service>-<process id>.json`, so two processes do not overwrite each other's records.
- `Recover` runs once at start-up, before anything is connected, in `bluelatch connect` and in the window. It switches on the services that files still describe and deletes the files. It leaves alone a file whose process is still running and is less than two minutes old, because that process is probably switching the service right now. A file for a device that is not paired any more is deleted.
- When switching a service on fails, the file stays, so the next start tries again.
- `list` does not recover, because it must stay read-only.

### Consequences

- A stop at any moment (the window closed, the process killed, a crash, a power loss) is repaired at the next start of `bluelatch connect` or of the window.
- One small file is written and deleted for every service of every request.
- When `%LocalAppData%` is not available there is no journal, and a warning is logged.
- Not covered: nothing runs between the stop and the next start, so the service stays switched off until then.

---

# 中文

## ADR 0005：用待恢复记录让后续运行重新打开被遗留关闭的服务

状态：2026-11-24 已接受。已在 0.1.1 版中构建。

### 背景

`Reconnect` 会把服务先关闭再打开（ADR 0001）。2026-11-24 一次真实运行（一台音箱）的日志说明了这意味着什么：

- 关闭 A2DP Sink 用了约 3.1 秒，服务关闭期间它从已安装服务列表中消失。打开它用了约 3.3 秒，调用返回时服务已回到列表中。
- 因此在整个请求的约 6.5 秒里，服务不是正在被关闭就是已经关闭；在这段时间里关闭窗口或杀掉进程，服务可能被留在关闭状态。此后列表里没有这个服务，`Reconnect` 也看不到它：如果它是该设备唯一的音频服务，就会报告该设备没有音频服务。

窗口让这种情况比命令行更容易发生。

备选方案：

1. 什么也不做，非正常停止后告诉用户重新配对设备。
2. 待恢复记录（journal）：关闭服务之前先写一个小文件，服务重新打开之后删除它，由下一次启动把文件仍然描述的服务重新打开。
3. 在一个不随父进程停止的辅助进程中执行每个请求。机制更多，而且辅助进程被杀时同样没有帮助。
4. 完全不关闭，只执行打开。不清楚这样是否能连上设备，所以不在设备上试过就无法选择它。

### 决定

采用方案 2，并在窗口中加一道保护：关闭窗口时先停止应用，并等待 Windows 正在处理的请求，最多 30 秒（`app.Shutdown`）。

- 待恢复记录是文件夹 `%LocalAppData%\BlueLatch\pending`。每次开关是一个文件 `pending-<address>-<service>-<process id>.json`，这样两个进程不会覆盖彼此的记录。
- `Recover` 在启动时、连接任何设备之前运行一次，在 `bluelatch connect` 和窗口中都会运行。它把文件仍然描述的服务重新打开，并删除文件。它不会处理这样的文件：写入它的进程仍在运行，并且文件不到两分钟，因为那个进程很可能正在开关这个服务。对于已经不再配对的设备，文件会被删除。
- 重新打开服务失败时，文件会保留，下一次启动再试。
- `list` 不执行恢复，因为它必须保持只读。

### 后果

- 任何时刻的停止（关闭窗口、进程被杀、崩溃、断电），都会在 `bluelatch connect` 或窗口下一次启动时得到修复。
- 每个请求的每个服务都会写入并删除一个小文件。
- `%LocalAppData%` 不可用时没有待恢复记录，并会记录一条警告。
- 没有覆盖的情况：从停止到下一次启动之间没有任何东西在运行，所以服务在那之前一直处于关闭状态。
