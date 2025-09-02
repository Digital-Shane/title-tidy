Output demo.gif

Set Framerate 10
Set Shell zsh
Set FontSize 16
Set Width 1450
Set Height 700

Hide
Type "./backup-config.sh"
Enter
Type "clear"
Enter
Type "cd ../shows"
Enter
Type "./make.sh"
Enter
Type "cd data"
Enter
Type "clear"
Enter
Show

Type "title-tidy config"
Sleep 1s
Enter
Sleep 250ms
Tab
Sleep 500ms
Tab
Sleep 500ms
Left 30
Sleep 250ms
Type "{title} - "
Sleep 1s
Ctrl+S
Sleep 1s
Ctrl+Q
Sleep 250ms

Type "title-tidy shows --no-nfo --no-img"
Sleep 1s
Enter
Sleep 500ms
Down
Sleep 500ms
Down
Sleep 500ms
Down
Sleep 500ms
Down
Sleep 500ms
Right
Sleep 500ms
Down
Sleep 500ms
Right
Sleep 500ms
Down
Sleep 500ms
Right
Sleep 500ms
Type "r"
Sleep 1s
Right
Sleep 500ms
Up
Sleep 500ms
Right
Sleep 500ms

Sleep 5s

Hide
Escape
Type "cd .."
Enter
Type "rm -rf data"
Enter
Type "cd ../config"
Enter
Type "./restore-config.sh"
Enter
