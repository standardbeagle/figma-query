const path = require('path');
const os = require('os');

/**
 * Get the path to the figma-query binary
 * @returns {string} Path to the binary
 */
function getBinaryPath() {
  const platform = os.platform();
  const binaryName = platform === 'win32' ? 'figma-query.exe' : 'figma-query';
  return path.join(__dirname, 'bin', binaryName);
}

/**
 * Get the command to run figma-query as an MCP server
 * @returns {{ command: string, args: string[] }}
 */
function getMcpCommand() {
  return {
    command: getBinaryPath(),
    args: [],
  };
}

module.exports = {
  getBinaryPath,
  getMcpCommand,
};
