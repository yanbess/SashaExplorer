# Reveal in SashaExplorer

Adds **Reveal in SashaExplorer** to the context menu of files and folders in the
VS Code Explorer and of editor tabs, and binds it to Shift+Alt+R, the keys of
"Reveal in File Explorer". A file opens with its folder and is selected.

SashaExplorer records its path for the current Windows user each time it runs,
so start it once before the first use, or set `sashaExplorer.path`.

Build the package with `python build_vsix.py`, then install it with
`code --install-extension reveal-in-sashaexplorer-0.1.0.vsix`.
