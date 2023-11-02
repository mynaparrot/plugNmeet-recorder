const args = process.argv.slice(2);

if (!args) {
  console.log('no option sent');
  process.exit();
}

// should get JSON value
console.log(args);

/*
 '{"recording_id":"RM_WiTK7pMya8Hc-1698923776909","room_table_id":841,"room_sid":"RM_WiTK7pMya8Hc","file_path":"../recording_files/node_01/RM_WiTK7pMya8Hc/RM_WiTK7pMya8Hc-1698923776909.mp4","file_size":0.32,"recorder_id":"node_01"}'
*/
