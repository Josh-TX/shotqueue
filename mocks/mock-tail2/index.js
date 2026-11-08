// Mock OBSBOT Tail 2 (direct OSC over UDP). No dependencies.
//   node index.js [--port=57110] [--reply-port=57120]
// Answers /OBSBOT/WebCam/General/Connected with GetGimbalPosInfoResp [pitch, yaw] (float32), and
// moves toward SetGimMotorDegree [speed, pan, pitch]. Replies go to <sender ip>:reply-port, like
// the real camera. shotqueue matches replies by source IP, so run only one mock per host address.
import dgram from 'node:dgram';

const arg = (n, d) => Number((process.argv.find((a) => a.startsWith(`--${n}=`)) ?? '').split('=')[1] ?? d);
const PORT = arg('port', 57110);
const REPLY_PORT = arg('reply-port', 57120);

let yaw = 0, pitch = 0, targetYaw = 0, targetPitch = 0, speed = 45;

const pad = (n) => (4 - (n % 4)) % 4;
const str = (s) => Buffer.concat([Buffer.from(s + '\0'), Buffer.alloc(pad(s.length + 1))]);

function encode(addr, floats) {
  const args = Buffer.alloc(floats.length * 4);
  floats.forEach((f, i) => args.writeFloatBE(f, i * 4));
  return Buffer.concat([str(addr), str(',' + 'f'.repeat(floats.length)), args]);
}

function readStr(buf, off) {
  const end = buf.indexOf(0, off);
  return [buf.toString('utf8', off, end), end + 1 + pad(end - off + 1)];
}

function decode(buf) {
  let [addr, off] = readStr(buf, 0);
  let tags;
  [tags, off] = readStr(buf, off);
  const args = [];
  for (const t of tags.slice(1)) {
    if (t === 'i') args.push(buf.readInt32BE(off));
    else if (t === 'f') args.push(buf.readFloatBE(off));
    else break;
    off += 4;
  }
  return { addr, args };
}

// Move toward the target at a rate based on speed (deg/sec = speed * 2), 20 ticks/sec.
setInterval(() => {
  const step = (speed * 2) / 20;
  const mv = (cur, tgt) => (Math.abs(tgt - cur) <= step ? tgt : cur + Math.sign(tgt - cur) * step);
  yaw = mv(yaw, targetYaw);
  pitch = mv(pitch, targetPitch);
}, 50);

const sock = dgram.createSocket('udp4');
sock.on('message', (msg, rinfo) => {
  let m;
  try { m = decode(msg); } catch { return; }
  console.log(`<- ${rinfo.address}:${rinfo.port} ${m.addr} ${JSON.stringify(m.args)}`);
  if (m.addr === '/OBSBOT/WebCam/General/Connected') {
    sock.send(encode('/OBSBOT/WebCam/General/GetGimbalPosInfoResp', [pitch, yaw]), REPLY_PORT, rinfo.address);
  } else if (m.addr === '/OBSBOT/WebCam/General/SetGimMotorDegree' && m.args.length === 3) {
    [speed, targetYaw, targetPitch] = m.args;
    if (speed <= 0) speed = 45;
  }
});
sock.bind(PORT, () => console.log(`mock-tail2 listening on UDP ${PORT}, replying to port ${REPLY_PORT}`));
