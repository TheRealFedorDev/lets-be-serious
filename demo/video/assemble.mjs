// frames.json -> ffconcat с длительностями -> mp4 1920x1080 30fps без звука
import fs from 'node:fs';
import { execFileSync } from 'node:child_process';
const [dir, out] = process.argv.slice(2);
const fr = JSON.parse(fs.readFileSync(dir + '/frames.json', 'utf8'));
let txt = 'ffconcat version 1.0\n';
for (let i = 0; i < fr.length; i++) {
  const d = i < fr.length - 1 ? Math.max(0.001, fr[i + 1].ts - fr[i].ts) : 1;
  txt += `file '${fr[i].file}'\nduration ${d.toFixed(4)}\n`;
}
txt += `file '${fr[fr.length - 1].file}'\n`;
fs.writeFileSync(dir + '/list.ffconcat', txt);
execFileSync('ffmpeg', ['-y', '-loglevel', 'error', '-f', 'concat', '-safe', '0', '-i', dir + '/list.ffconcat',
  '-vf', 'fps=60,scale=1920:1080:flags=lanczos,format=yuv420p', '-c:v', 'libx264', '-preset', 'slow', '-crf', '17',
  '-profile:v', 'high', '-movflags', '+faststart', '-an', out], { stdio: 'inherit' });
console.log('ok', out, ((fr[fr.length - 1].ts - fr[0].ts)).toFixed(1) + 's');
