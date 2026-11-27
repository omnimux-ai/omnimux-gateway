const { contextBridge, ipcRenderer } = require('electron');

// 获取数据目录路径（用于显示给用户）
function getDataDirPath() {
  try {
    const dir = ipcRenderer.sendSync('omnimux:get-data-dir');
    if (dir) {
      return dir;
    }
  } catch {
    // Fall back when running outside the main IPC handler.
  }
  if (typeof process !== 'undefined' && process.env && process.env.ELECTRON_DATA_DIR) {
    return process.env.ELECTRON_DATA_DIR;
  }
  return undefined;
}

contextBridge.exposeInMainWorld('electron', {
  isElectron: true,
  version: process.versions.electron,
  platform: process.platform,
  versions: process.versions,
  dataDir: getDataDirPath()
});