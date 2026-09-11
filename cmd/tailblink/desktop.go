package main

type desktopUI struct {
	done  <-chan error
	close func()
}

func (ui desktopUI) Done() <-chan error { return ui.done }

func (ui desktopUI) Close() {
	if ui.close != nil {
		ui.close()
	}
}
