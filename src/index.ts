import yaml from 'js-yaml';
import * as fs from 'fs';

import { logger } from './utils/helper';
import PNMRecorder from './PNMRecorder';

let config: any, pnm: PNMRecorder;
try {
  config = yaml.load(fs.readFileSync('config.yaml', 'utf8'));
} catch (e) {
  console.error('Error: ', e);
  process.exit();
}

process.on('SIGINT', async () => {
  logger.info('Caught interrupt signal, cleaning up');
  if (typeof pnm === 'undefined') {
    process.exit();
  }

  pnm.childProcessesMapByRoomSid.forEach((c) => c.kill('SIGINT'));
  //await sleep(5000);

  // clear everything
  pnm.childProcessesMapByRoomSid.clear();
  pnm.childProcessesInfoMapByChildPid.clear();
  // now end the process
  process.exit();
});

(async () => {
  // check for logs directory
  const logDir = './logs';
  if (!fs.existsSync(logDir)) {
    fs.mkdirSync(logDir);
  }

  pnm = new PNMRecorder(config);
  await pnm.openNatsConnection();
})();
