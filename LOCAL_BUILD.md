# 本机 macOS 构建与启动

本机运行的 OpenList 二进制放在 `output/local/openlist`，由仓库已有的 `output/` 规则排除，不提交到 Git。运行数据仍在 `~/.local/share/openlist`。

在本仓库执行 `./build-local-macos.sh`。脚本将新产物先写到临时文件，构建成功后替换 `output/local/openlist`；默认版本标签与现役产物相同，为 `4.2.9`，可通过 `OPENLIST_VERSION` 和 `OPENLIST_WEB_VERSION` 覆盖。上游跨平台发布脚本 `build.sh` 保持原用途。

本机 LaunchAgent `~/Library/LaunchAgents/org.openlist.plist` 的程序路径应为 `~/AIProjects/openlist-quark-fix/output/local/openlist` 的绝对路径。旧的 `~/AIProjects/openlist-patched-build/openlist` 路径已移除。重新构建后可通过 `launchctl kickstart -k gui/$(id -u)/org.openlist` 重启服务；先确认新产物可执行，再重启。
