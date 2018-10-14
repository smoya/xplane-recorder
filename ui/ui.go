package ui

import (
	"log"

	"github.com/andlabs/ui"
	_ "github.com/andlabs/ui/winmanifest"
)

var mainwin *ui.Window

func Initialize(startFunc, stopFunc func() error, saveFunc func(flightName, filename string) error) error {
	return ui.Main(func() {
		mainwin = ui.NewWindow("XPlane 11 Flight recorder - ©2018 Sergio Moya https://sergiomoya.me", 500, 150, true)
		mainwin.OnClosing(func(*ui.Window) bool {
			ui.Quit()
			return true
		})
		ui.OnShouldQuit(func() bool {
			mainwin.Destroy()
			return true
		})

		mainwin.SetChild(mainPage(startFunc, stopFunc, saveFunc))
		mainwin.SetMargined(true)

		mainwin.Show()
	})
}

func mainPage(startFunc, stopFunc func() error, saveFunc func(flightName, filename string) error) ui.Control {
	saveToKMLButton := ui.NewButton("Save .kml file (Google Earth)")
	saveToKMLButton.Disable()
	saveToKMLButton.OnClicked(func(*ui.Button) {
		filename := ui.SaveFile(mainwin)
		if filename == "" {
			// TODO
			log.Println("No filename provided!")
		}

		err := saveFunc("FOO-BAR", filename)
		if err != nil {
			// TODO
			log.Println("error on save kml file: ", err.Error())
		}

	})

	statusLabel := ui.NewLabel("Status: NO Logging")
	startButton := ui.NewButton("Start logging")
	stopButton := ui.NewButton("Stop logging")
	stopButton.Disable()

	startButton.OnClicked(func(b *ui.Button) {
		b.Disable()
		stopButton.Enable()
		statusLabel.SetText("Status: Logging")
		saveToKMLButton.Disable()
		err := startFunc()
		if err != nil {
			log.Println("error on start: ", err.Error())
		}
	})

	stopButton.OnClicked(func(b *ui.Button) {
		b.Disable()
		startButton.Enable()
		statusLabel.SetText("Status: NO Logging")
		saveToKMLButton.Enable()

		err := stopFunc()
		if err != nil {
			log.Println("error on stop: ", err.Error())
		}
	})

	mainVbox := ui.NewVerticalBox()
	mainVbox.SetPadded(true)

	hbox := ui.NewHorizontalBox()
	hbox.SetPadded(true)
	mainVbox.Append(hbox, false)

	buttonsVbox := ui.NewVerticalBox()
	buttonsVbox.SetPadded(true)
	hbox.Append(buttonsVbox, false)

	buttonsVbox.Append(startButton, false)
	buttonsVbox.Append(stopButton, false)

	hbox.Append(ui.NewVerticalSeparator(), false)

	saveVbox := ui.NewVerticalBox()
	saveVbox.SetPadded(true)
	saveVbox.Append(saveToKMLButton, false)

	hbox.Append(saveVbox, true)

	mainVbox.Append(ui.NewHorizontalSeparator(), false)
	mainVbox.Append(statusLabel, false)

	return mainVbox
}
