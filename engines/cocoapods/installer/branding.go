package installer

import (
	"bytes"
	"io"
	"os"
)

// podCompletion is how CocoaPods opens the summary it prints at the end
// of every install (Pod::Installer#print_post_install_message): "Pod
// installation complete! There are 3 dependencies from the Podfile and 5
// total pods installed." `pod install` and `pod update` both print it,
// and so does the native installer, which drives the same Pod::Installer.
const podCompletion = "Pod installation complete!"

// brandedOutput passes CocoaPods' output through line by line, unchanged
// except that the completion summary opens with Zorvok's own words
// ("Zorvok install complete! There are 3 dependencies from the Podfile
// and 5 total pods installed."), so the last thing `zorvok install` says
// is about Zorvok. The rest of the line, its colors included, and every
// other line are CocoaPods' own output, byte for byte.
type brandedOutput struct {
	w       io.Writer
	replace []byte
	pending []byte
}

func newBrandedOutput(w io.Writer, replace string) *brandedOutput {
	return &brandedOutput{w: w, replace: []byte(replace)}
}

func (b *brandedOutput) Write(p []byte) (int, error) {
	b.pending = append(b.pending, p...)
	for {
		i := bytes.IndexByte(b.pending, '\n')
		if i < 0 {
			return len(p), nil
		}
		if err := b.emit(b.pending[:i+1]); err != nil {
			return 0, err
		}
		b.pending = b.pending[i+1:]
	}
}

// Flush writes out a last line that didn't end in a newline.
func (b *brandedOutput) Flush() error {
	if len(b.pending) == 0 {
		return nil
	}
	err := b.emit(b.pending)
	b.pending = nil
	return err
}

func (b *brandedOutput) emit(line []byte) error {
	_, err := b.w.Write(bytes.Replace(line, []byte(podCompletion), b.replace, 1))
	return err
}

// podArgs builds `pod <command> <extraArgs...>` for output going to out.
// Through brandedOutput, `pod` writes to a pipe rather than the user's
// terminal, and CocoaPods turns its colors off for a pipe. When out is
// the terminal, --ansi keeps them on, unless the user already chose with
// --ansi or --no-ansi.
func podArgs(out io.Writer, command string, extraArgs []string) []string {
	args := append([]string{command}, extraArgs...)
	for _, a := range extraArgs {
		if a == "--ansi" || a == "--no-ansi" {
			return args
		}
	}
	if isTerminal(out) {
		args = append(args, "--ansi")
	}
	return args
}

func isTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	info, err := f.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}
