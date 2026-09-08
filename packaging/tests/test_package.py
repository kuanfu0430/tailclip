"""從 Windows 建置時仍應交付可直接在 Linux 使用的壓縮檔。"""
import importlib.util
import io
from pathlib import Path
import tarfile
import tempfile
import unittest
from unittest.mock import patch

spec = importlib.util.spec_from_file_location('tailclip_package', Path(__file__).resolve().parents[1] / 'package.py')
package = importlib.util.module_from_spec(spec)
spec.loader.exec_module(package)


class PackageTests(unittest.TestCase):
    def test_linux_archive_permissions_and_line_endings(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            package.write_text(root / 'CLOUDFLARED.txt', 'tailclip-cloudflared-0123456789ab\n')
            (root / 'tailclip').write_bytes(b'ELF')
            (root / 'install.sh').write_bytes(b'#!/bin/bash\n')
            self.assertEqual((root / 'CLOUDFLARED.txt').read_bytes(), b'tailclip-cloudflared-0123456789ab\n')
            stream = io.BytesIO()
            with tarfile.open(fileobj=stream, mode='w') as archive:
                for path in root.iterdir():
                    info = archive.gettarinfo(path, arcname=path.name)
                    info.mode = 0o666  # 模擬 Windows stat 無執行權限。
                    with path.open('rb') as data:
                        archive.addfile(package.linux_entry(info), data)
            stream.seek(0)
            with tarfile.open(fileobj=stream) as archive, patch.object(package, 'verify_files'):
                package.verify_linux_archive(archive)
                self.assertEqual(archive.getmember('tailclip').mode, 0o755)
                self.assertEqual(archive.getmember('install.sh').mode, 0o755)
                self.assertEqual(archive.getmember('CLOUDFLARED.txt').mode, 0o644)
                archive.getmember('tailclip').mode = 0o666
                with self.assertRaisesRegex(AssertionError, '權限'):
                    package.verify_linux_archive(archive)

    def test_linux_archive_rejects_crlf(self):
        stream = io.BytesIO()
        with tarfile.open(fileobj=stream, mode='w') as archive:
            data = b'tailclip-cloudflared-0123456789ab\r\n'
            info = tarfile.TarInfo('CLOUDFLARED.txt')
            info.size, info.mode = len(data), 0o644
            archive.addfile(info, io.BytesIO(data))
        stream.seek(0)
        with tarfile.open(fileobj=stream) as archive, patch.object(package, 'verify_files'):
            with self.assertRaisesRegex(AssertionError, 'LF'):
                package.verify_linux_archive(archive)


if __name__ == '__main__':
    unittest.main()
