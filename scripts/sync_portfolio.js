const fs = require('fs');
const path = require('path');
const { execSync } = require('child_process');

const mtlgSiteDir = 'B:/workgit/mtlg-site';
const mtlglabsDir = 'B:/workgit/mtlglabs-space';

const imageList = [
  'images/previewforge_demo.gif',
  'images/previewforge_dashboard.png',
  'images/previewforge_logs_exec.png',
  'images/previewforge_exec_console.png',
  'images/previewforge_mobile.png',
];

// 1. Update project_images_map.json in both repos
[mtlgSiteDir, mtlglabsDir].forEach((repoDir) => {
  const mapFile = path.join(repoDir, 'project_images_map.json');
  if (fs.existsSync(mapFile)) {
    const map = JSON.parse(fs.readFileSync(mapFile, 'utf8'));
    map.previewforge = imageList;
    fs.writeFileSync(mapFile, JSON.stringify(map, null, 2) + '\n');
    console.log(`Updated ${mapFile}`);
  }
});

// 2. Update projects.json in both repos
[mtlgSiteDir, mtlglabsDir].forEach((repoDir) => {
  const pFile = path.join(repoDir, 'projects.json');
  if (fs.existsSync(pFile)) {
    const list = JSON.parse(fs.readFileSync(pFile, 'utf8'));
    const item = list.find((x) => x.id === 'previewforge');
    if (item) {
      item.badgeLabs = { text: 'Live', type: 'live' };
      item.badgeSite = { text: 'Live', type: 'live' };
      item.image = imageList[0];
      item.images = imageList;
      item.desc = 'Self-hosted ephemeral PR preview supervisor that deploys isolated Docker Compose stacks from GitHub webhooks with automatic HTTPS subdomains and scale-to-zero hibernation.';
      fs.writeFileSync(pFile, JSON.stringify(list, null, 2) + '\n');
      console.log(`Updated ${pFile}`);
    }
  }
});

// 3. Run sync.js in mtlg-site
console.log('Running sync.js in mtlg-site...');
execSync('node sync.js', { cwd: mtlgSiteDir, stdio: 'inherit' });

// 4. Run compare_projects.js
console.log('Running compare_projects.js...');
execSync('node compare_projects.js', { cwd: mtlgSiteDir, stdio: 'inherit' });

console.log('Parity verified successfully!');
