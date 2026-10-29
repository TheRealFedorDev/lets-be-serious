#!/bin/zsh
# Параллельная запись 4 кусков и склейка встык. Использование: ./parallel.sh
cd "${0:A:h}"
typeset -A G
G[1]="intro,home"; G[2]="search,hotel,booking"; G[3]="trip,share"; G[4]="assistant,outro"
for i in 1 2 3 4; do ( node record.mjs "$PWD/g$i" "${G[$i]}" > "g$i.log" 2>&1 && node assemble.mjs "$PWD/g$i" "$PWD/g$i.mp4" >> "g$i.log" 2>&1 ) & done
wait
printf "file %s\n" g1.mp4 g2.mp4 g3.mp4 g4.mp4 > concat.txt
ffmpeg -loglevel error -y -f concat -safe 0 -i concat.txt -c copy -movflags +faststart final_par.mp4 && echo OK
