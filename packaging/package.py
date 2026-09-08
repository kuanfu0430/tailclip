#!/usr/bin/env python3
"""用相同腳本產生本機及 CI 發行包，釘選 cloudflared 並核對全部檔案。"""
import argparse, hashlib, io, json, os, pathlib, shutil, subprocess, tarfile, urllib.request, zipfile
ROOT=pathlib.Path(__file__).resolve().parents[1]
SHORTCUTS=['TailClip-Send.shortcut','TailClip-Pull.shortcut','TailClip-Simple-Send.shortcut','TailClip-Simple-Pull.shortcut']

def digest(data):return hashlib.sha256(data).hexdigest()
def write_text(path,text):
    # 固定 UTF-8／LF，Windows 建置的 Linux 套件亦可直接使用。
    path.write_text(text,encoding='utf-8',newline='\n')
def dependency(platform):
    lock=json.loads((ROOT/'internal/tunnel/cloudflared.json').read_text())['assets'][platform]
    cache=ROOT/'build/deps'/lock['filename'];cache.parent.mkdir(parents=True,exist_ok=True)
    if not cache.exists() or digest(cache.read_bytes())!=lock['sha256']:
        archive=urllib.request.urlopen(lock['url'],timeout=120).read()
        assert digest(archive)==lock['archive_sha256'],'cloudflared 下載校驗失敗'
        if lock['url'].endswith('.tgz'):
            with tarfile.open(fileobj=io.BytesIO(archive)) as t:archive=t.extractfile('cloudflared').read()
        assert digest(archive)==lock['sha256'],'cloudflared 執行檔校驗失敗'
        cache.write_bytes(archive)
    cache.chmod(0o755)
    return cache

def verify_files(files):
    rows=files['SHA256SUMS.txt'].decode().splitlines()
    expected={line.split('  ',1)[1]:line.split('  ',1)[0] for line in rows}
    assert set(expected)==set(files)-{'SHA256SUMS.txt'},'封裝檔案未完整列入 checksum'
    for name,sha in expected.items():assert digest(files[name])==sha,name
    for name in SHORTCUTS:assert files[name]==(ROOT/'shortcuts/dist'/name).read_bytes(),name
    companion=files['CLOUDFLARED.txt'].decode().strip()
    assert companion in files and files[companion], '缺少隧道程式'
    locks=json.loads((ROOT/'internal/tunnel/cloudflared.json').read_text())['assets'].values()
    lock=next(item for item in locks if item['filename']==companion)
    assert digest(files[companion])==lock['sha256'],'封裝隧道版本不符'

def executable(name):
    return name in ('tailclip','TailClip.exe') or name.startswith('tailclip-cloudflared-') or name.endswith('.sh')

def linux_entry(info):
    # 不沿用 Windows stat 的權限；tar 格式才是 Linux 安裝時的依據。
    info.mode=0o755 if executable(info.name) else 0o644
    return info

def verify_linux_archive(archive):
    files={m.name:archive.extractfile(m).read() for m in archive.getmembers()}
    verify_files(files)
    for m in archive.getmembers():
        assert m.mode==(0o755 if executable(m.name) else 0o644),f'Linux 檔案權限不符：{m.name}'
        if m.name.endswith(('.txt','.sh','.service')):
            assert b'\r' not in files[m.name],f'Linux 文字檔必須使用 LF：{m.name}'

def package(version,platform,out):
    stage=ROOT/'build/package'/platform
    if stage.exists():shutil.rmtree(stage)
    stage.mkdir(parents=True)
    windows=platform=='windows-amd64'
    binary='TailClip.exe' if windows else 'tailclip'
    env=os.environ.copy();env.update(GOOS='windows' if windows else 'linux',GOARCH='amd64',CGO_ENABLED='0')
    flags=f'-s -w -X github.com/kuanfu0430/tailclip/internal/buildinfo.Version={version}'
    if windows:flags+=' -H=windowsgui'
    subprocess.run(['go','build','-trimpath','-ldflags',flags,'-o',str(stage/binary),'./cmd/tailclip'],cwd=ROOT,env=env,check=True)
    dep=dependency(platform);shutil.copy2(dep,stage/dep.name)
    write_text(stage/'CLOUDFLARED.txt',dep.name+'\n')
    shutil.copy2(ROOT/'packaging/LICENSE-cloudflared.txt',stage/'LICENSE-cloudflared.txt')
    for name in SHORTCUTS:shutil.copy2(ROOT/'shortcuts/dist'/name,stage/name)
    names=['README-Windows.txt','Start-TailClip.cmd','Uninstall-TailClip.cmd'] if windows else ['install.sh','uninstall.sh','tailclip.service']
    for name in names:shutil.copy2(ROOT/'packaging'/('windows' if windows else 'linux')/name,stage/name)
    if not windows:
        for file in stage.iterdir():
            if file.suffix in ('.txt','.sh','.service'):
                write_text(file,file.read_text(encoding='utf-8'))
    write_text(stage/'VERSION.txt',version+'\n')
    write_text(stage/'SOURCE.txt',subprocess.check_output(['git','rev-parse','HEAD'],cwd=ROOT).decode())
    for file in stage.iterdir():
        if file.name==binary or file.name==dep.name or file.suffix=='.sh':file.chmod(0o755)
    write_text(stage/'SHA256SUMS.txt',''.join(f'{digest(p.read_bytes())}  {p.name}\n' for p in sorted(stage.iterdir())))
    out.mkdir(parents=True,exist_ok=True)
    if windows:
        result=out/f'TailClip-{version}-windows-x64.zip'
        with zipfile.ZipFile(result,'w',zipfile.ZIP_DEFLATED,compresslevel=9) as z:
            for p in sorted(stage.iterdir()):z.write(p,p.name)
        with zipfile.ZipFile(result) as z:verify_files({n:z.read(n) for n in z.namelist()})
    else:
        result=out/f'TailClip-{version}-linux-x64.tar.gz'
        with tarfile.open(result,'w:gz') as t:
            for p in sorted(stage.iterdir()):t.add(p,arcname=p.name,filter=linux_entry)
        with tarfile.open(result) as t:verify_linux_archive(t)
    write_text(result.with_name(result.name+'.sha256'),digest(result.read_bytes())+'  '+result.name+'\n')
    print('已驗證發行包：',result,flush=True)
    return result

def main():
    p=argparse.ArgumentParser();p.add_argument('--version',required=True);p.add_argument('--output',type=pathlib.Path,default=ROOT/'dist');p.add_argument('--platform',choices=['windows-amd64','linux-amd64','all'],default='all');a=p.parse_args()
    if not __import__('re').fullmatch(r'[A-Za-z0-9][A-Za-z0-9._-]*',a.version):p.error('版本字串不合法')
    for platform in (['windows-amd64','linux-amd64'] if a.platform=='all' else [a.platform]):package(a.version,platform,a.output)
if __name__=='__main__':main()
