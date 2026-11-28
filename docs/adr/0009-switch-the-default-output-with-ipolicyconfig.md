# English

## ADR 0009: The default output is changed with the undocumented IPolicyConfig interface

Status: accepted on 2026-11-28, after the user asked for a way to switch the sound output (0.1.4) and answered the design questions below. Built in version 0.1.4.

### Context

The user switches between a Bluetooth speaker and the speakers of a monitor, and wants to do it from BlueLatch instead of the sound flyout of Windows. The window and the command line should list the outputs Windows has and change the default one.

Windows documents how to list the outputs and read the defaults (the Core Audio API, `mmdeviceapi.h`), but has no documented call that sets the default output. Options:

1. The `IPolicyConfig` interface of the class `CPolicyConfigClient`: undocumented, and what utilities that switch the default output commonly use. It needs no administrator rights.
2. Open the sound flyout or the Sound page of Windows Settings and leave the click to the user. Documented, but it is not a switch.
3. Drive the flyout with simulated key presses or UI Automation. Fragile, and it takes the focus away.
4. Run another program that switches (NirCmd, a PowerShell module). A dependency on something that has to be installed.

For option 1 the COM calls are made through the method tables with `golang.org/x/sys/windows` and `ole32.dll`, without cgo and without a new dependency (AGENTS.md, section 10).

### Decision

Use option 1, in a new package, `internal/audio`.

The user chose these rules:

- The window lists every active output, with the current one marked and a **Switch** button on each of the others.
- The console and multimedia roles get the new output, which is the one Windows shows as selected. The communications role (calls) gets it only when it is on the current default output already; a separate one is left alone.
- The command line has `bluelatch switch [<name or ID>]`: to the named output, or without a name to the one after the current one.

Safeguards:

- The default is changed only on a click on **Switch** or by `bluelatch switch`, never by itself.
- Only a role that does not have the output yet is set, and the defaults are read again afterwards. When Windows did not change one, that is reported (`audio.ErrNotSwitched`).
- When the interface cannot be created, that is reported (`audio.ErrUnsupported`), and the rest of the program works.
- The position of `SetDefaultEndpoint` in the method table (13) was checked against the [`PolicyConfig.h` of ohSongcast](https://github.com/openhome/ohSongcast/blob/master/ohSongcast/Windows/PolicyConfig.h), which declares the same interface and class IDs. A wrong position would call another method.

### Observed

On 2026-11-28 on the development PC (Windows 11 Enterprise LTSC 10.0.26100), from a process that was not elevated, with the permission of the user:

- The two active outputs, a Bluetooth speaker and the speakers of a monitor, were read, with the three roles on the speaker.
- `bluelatch switch` moved all three roles to the monitor, and a second `bluelatch switch` with the name of the speaker moved them back. Each time the defaults were read again afterwards and showed the new output at once, and the exit code was 0. The two switches were less than a second apart.

### Consequences

- A future update of Windows may change or remove the interface. The failure is then an error message; the outputs are still listed, and the flyout of Windows still works.
- The rule for calls looks only at what is set now and remembers nothing. Calls that were set apart, and are switched away from and back, can end up following the default: when the default is switched to the output that has the calls, the two are the same, and from then on the calls follow.
- Only the default output is changed. An output that an app has been given by itself in the volume mixer is not touched, and neither is the input (microphone).
- A switch runs at once and switches nothing off, so it needs no record in the journal, and closing the window waits only for it to return.

---

# 中文

## ADR 0009：用未公开的 IPolicyConfig 接口改变默认输出

状态：2026-11-28 在用户要求切换声音输出的功能（0.1.4）并回答了下面的设计问题之后接受。已在 0.1.4 版中构建。

### 背景

用户在蓝牙音箱和显示器的扬声器之间切换，希望能在 BlueLatch 里完成，而不是去 Windows 的声音弹出面板。窗口和命令行应该列出 Windows 的输出，并能改变默认输出。

Windows 公开了列出输出和读取默认项的方法（Core Audio API，`mmdeviceapi.h`），但没有设置默认输出的公开调用。备选方案：

1. `CPolicyConfigClient` 类的 `IPolicyConfig` 接口：未公开，切换默认输出的工具普遍使用它。不需要管理员权限。
2. 打开声音弹出面板或 Windows 设置的声音页面，让用户自己点。有公开接口，但这不算切换。
3. 用模拟按键或 UI Automation 操作弹出面板。脆弱，而且会抢走焦点。
4. 运行另一个能切换的程序（NirCmd、某个 PowerShell 模块）。依赖必须另行安装的东西。

对方案 1，通过 `golang.org/x/sys/windows` 和 `ole32.dll` 按方法表发起 COM 调用，不使用 cgo，也不增加依赖（AGENTS.md 第 10 节）。

### 决定

采用方案 1，放在新包 `internal/audio` 中。

用户选定了这些规则：

- 窗口列出每一个可用的输出，当前输出打标记，其他每个输出有一个 **Switch** 按钮。
- 控制台（console）和多媒体（multimedia）角色换到新输出，它就是 Windows 显示为已选中的那个。通信（communications，通话用）角色只有当它本来就在当前默认输出上时才跟着换；单独指定的保持不动。
- 命令行有 `bluelatch switch [<name or ID>]`：切到指定的输出；不给名称时切到当前输出的下一个。

保护措施：

- 默认输出只在点击 **Switch** 或执行 `bluelatch switch` 时改变，绝不自行改变。
- 只设置还不在该输出上的角色，设置后重新读取默认项。Windows 没有改变某一个时会报告出来（`audio.ErrNotSwitched`）。
- 接口无法创建时报告出来（`audio.ErrUnsupported`），程序其余部分照常工作。
- `SetDefaultEndpoint` 在方法表中的位置（13）对照 [ohSongcast 的 `PolicyConfig.h`](https://github.com/openhome/ohSongcast/blob/master/ohSongcast/Windows/PolicyConfig.h) 核对过，它声明了相同的接口 ID 和类 ID。位置错了就会调用到别的方法。

### 观察到的情况

2026-11-28 在开发用电脑上（Windows 11 Enterprise LTSC 10.0.26100），从未提权的进程中，经用户许可：

- 读到两个可用输出：一台蓝牙音箱和一台显示器的扬声器，三个角色都在音箱上。
- `bluelatch switch` 把三个角色都换到了显示器，随后用音箱名称执行的第二次 `bluelatch switch` 又把它们换了回来。每次之后重新读取默认项，立刻就显示新的输出，退出码为 0。两次切换相隔不到一秒。

### 后果

- 将来的 Windows 更新可能改变或移除这个接口。届时的表现是一条错误消息；输出仍然会列出，Windows 的弹出面板也照常可用。
- 通话的规则只看当前的设置，不记忆任何东西。单独指定过的通话设备，在切走又切回之后，可能变成跟随默认输出：当默认输出切到承载通话的那个输出时，两者相同，从此通话就跟着走了。
- 只改变默认输出。应用在音量混合器里单独指定的输出不受影响，输入（麦克风）也不受影响。
- 切换立即完成，也不关闭任何东西，所以不需要待恢复记录，关闭窗口时也只需等它返回。
