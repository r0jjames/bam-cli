package view

// EditorCommand picks the editor for bam run --edit: BAM_EDITOR, the machine
// file's editor, VISUAL, EDITOR, then vi (notepad on Windows). It mirrors
// PagerCommand; the command is split on whitespace, so "code --wait" works.
func EditorCommand(getenv func(string) string, machineEditor, goos string) string {
	for _, v := range []string{getenv("BAM_EDITOR"), machineEditor, getenv("VISUAL"), getenv("EDITOR")} {
		if v != "" {
			return v
		}
	}
	if goos == "windows" {
		return "notepad"
	}
	return "vi"
}
