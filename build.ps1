# compile the go code into a binary called "VRChive", package it into a zip file with the data folder

go build -o VRChive.exe

# use 7z to compress the folder ./data and the binary into a zip file
& 'C:\Program Files\7-Zip\7z.exe' a -t7z -mx9 -r VRChive.zip ./data ./VRChive.exe

