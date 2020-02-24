package format

import "github.com/gookit/color"

var (
	//Add colorizes the add field
	Add = color.LightGreen.Render
	//Update renders the update field
	Update = color.LightYellow.Render
	//Remove renders the remove field
	Remove = color.Error.Render
	//Notice renders something Notice
	Notice = color.Notice.Render
	//Green renders something Green
	Green = color.Green.Render
	//LightGreen renders something LightGreen
	LightGreen = color.LightGreen.Render
)
