#!/usr/bin/env node
// Synthetic subprocess boundary fixture, never a substitute for the real Go health test.
import http from 'node:http';
const raw = 'C29_SECRET_SENTINEL password=C29_CREDENTIAL_SENTINEL /private/C29_PATH_SENTINEL\n';
process.stderr.write(raw.repeat(10000));
if (process.env.ATTESTARA_C29_FIXTURE === 'oversized') {
  process.stdout.write(raw.repeat(10000));
} else if (process.env.ATTESTARA_C29_FIXTURE === 'malformed') {
  process.stdout.write('{"event":"listening","address":"' + raw + '"}\n');
} else {
  const server = http.createServer((req, res) => {
    res.setHeader('Content-Type', 'application/json');
    res.end(process.env.ATTESTARA_C29_FIXTURE === 'invalid_health' ? JSON.stringify({secret:raw}) : JSON.stringify({schemaVersion:1,status:'ok',scope:'engine-process',version:'0.0.1',goVersion:'go1.27.2'}));
  });
  server.listen(0, '127.0.0.1', () => {
    process.stdout.write(JSON.stringify({event:'listening',address:'127.0.0.1:' + server.address().port}) + '\n');
    setTimeout(() => { process.stdout.write(raw.repeat(10000)); }, 100);
  });
}
