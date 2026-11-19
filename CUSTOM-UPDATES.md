# 定制版更新

定制代码以 `mygo-custom` 为准。安装程序在「设置 → 更新」里可以检查两个上游是否还有待合并的提交，也可以检查播放器和网易云插件的定制版发行。

`Sync upstream` 每天在 Fork 的默认分支运行，也可以手动运行。有新提交时会创建指向 `mygo-custom` 的 PR，并在独立任务中尝试合并、测试和打包。检查工作流运行对应的提交，再使用 **Create a merge commit** 合并 PR，以保留上游提交历史。发生冲突时，手动合并 `upstream/main` 并验证后再发布。

两个 Fork 的 `Draft custom release` 均为手动工作流，从 `mygo-custom` 构建。输入比已发布版本更高的版本号，播放器也需要填写更新说明。工作流只创建草稿。确认产物后，在 GitHub Releases 发布草稿；设置页随后才能检到并安装更新。Windows 播放器使用 `custom-v` 标签和签名更新包，网易云插件使用 `v` 标签。

播放器签名密钥需在 Fork 的 Actions 密钥中命名为 `MYGO_UPDATER_PRIVATE_KEY`，公钥随播放器构建。网易云插件必须从定制 Fork 安装，设置页才会提供对应插件的更新。
