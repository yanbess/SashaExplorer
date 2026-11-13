// Reveal in SashaExplorer: shows a file or folder in SashaExplorer, the way
// "Reveal in File Explorer" shows it in Windows Explorer. A file opens with its
// folder and is selected there.
const vscode = require('vscode')
const { execFile, spawn } = require('child_process')
const fs = require('fs')

/** Where SashaExplorer is: the setting, or the path the program records. */
function locate() {
  const configured = vscode.workspace.getConfiguration('sashaExplorer').get('path')
  if (configured) return Promise.resolve(configured)
  return new Promise((resolve) => {
    execFile('reg', ['query', 'HKCU\Software\SashaExplorer', '/v', 'Path'], { windowsHide: true }, (error, stdout) => {
      const match = !error && /Path\s+REG_SZ\s+(.+)/.exec(stdout)
      resolve(match ? match[1].trim() : '')
    })
  })
}

/** The item to show: the one clicked, else the open file, else the folder. */
function target(uri) {
  if (uri && uri.scheme === 'file') return uri.fsPath
  const editor = vscode.window.activeTextEditor
  if (editor && editor.document.uri.scheme === 'file') return editor.document.uri.fsPath
  const folder = vscode.workspace.workspaceFolders && vscode.workspace.workspaceFolders[0]
  return folder ? folder.uri.fsPath : ''
}

async function reveal(uri) {
  const path = target(uri)
  if (!path) {
    vscode.window.showWarningMessage('There is no file or folder to show.')
    return
  }
  const exe = await locate()
  if (!exe || !fs.existsSync(exe)) {
    vscode.window.showErrorMessage(
      'SashaExplorer was not found. Start it once, or set "sashaExplorer.path" in the settings.',
    )
    return
  }
  // SashaExplorer runs one window, so this hands the path to it and ends.
  const child = spawn(exe, [path], { detached: true, stdio: 'ignore', windowsHide: false })
  child.on('error', (error) => vscode.window.showErrorMessage(`SashaExplorer could not start: ${error.message}`))
  child.unref()
}

function activate(context) {
  context.subscriptions.push(vscode.commands.registerCommand('sashaExplorer.reveal', reveal))
}

function deactivate() {}

module.exports = { activate, deactivate }
