import yaml from 'js-yaml';
import * as fs from 'fs';
import { fork } from 'child_process';
import { logger, sleep } from './utils/helper';
import PNMRecorder from './PNMRecorder';

let config: any;
try {
  config = yaml.load(fs.readFileSync('config.yaml', 'utf8'));
} catch (e) {
  console.error('Error: ', e);
  process.exit();
}

process.on('SIGINT', async () => {
  logger.info('Caught interrupt signal, cleaning up');

  /*childProcessesMapByRoomSid.forEach((c) => c.kill('SIGINT'));
  await sleep(5000);

  if (redis && redis.status === 'connect') {
    redis.disconnect();
    subNode.disconnect();
  }
  // clear everything
  childProcessesMapByRoomSid.clear();
  childProcessesInfoMapByChildPid.clear();*/
  // now end the process
  process.exit();
});

(async () => {
  console.log(config);
  const pnm = new PNMRecorder(config);
  await pnm.openNatsConnection();
})();
