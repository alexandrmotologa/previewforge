const path = require('path');
const fs = require('fs');
const { execSync } = require('child_process');
const puppeteer = require('./node_modules/puppeteer-core');

const CHROME_PATH = 'C:\\Program Files\\Google\\Chrome\\Application\\chrome.exe';
const FRAMES_DIR = path.join(__dirname, '..', 'docs', 'images', 'frames');
const OUTPUT_GIF = path.join(__dirname, '..', 'docs', 'images', 'previewforge_demo.gif');

async function sleep(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

async function record() {
  if (!fs.existsSync(FRAMES_DIR)) {
    fs.mkdirSync(FRAMES_DIR, { recursive: true });
  }

  // Clear previous frames
  fs.readdirSync(FRAMES_DIR).forEach((f) => {
    if (f.endsWith('.png')) fs.unlinkSync(path.join(FRAMES_DIR, f));
  });

  console.log('Launching Chrome for UI recording...');
  const browser = await puppeteer.launch({
    executablePath: CHROME_PATH,
    headless: 'new',
    defaultViewport: {
      width: 1280,
      height: 760,
      deviceScaleFactor: 1.5,
    },
    args: ['--hide-scrollbars', '--no-sandbox'],
  });

  const page = await browser.newPage();
  await page.goto('http://localhost:9850', { waitUntil: 'networkidle0' });
  await sleep(600);

  let frameIdx = 0;
  async function snap(name) {
    frameIdx++;
    const filename = `frame_${String(frameIdx).padStart(2, '0')}.png`;
    const filepath = path.join(FRAMES_DIR, filename);
    await page.screenshot({ path: filepath });
    console.log(`[Frame ${frameIdx}] ${name} -> ${filename}`);
  }

  // 1. Initial Dashboard Overview
  await snap('Dashboard Overview');
  await sleep(400);

  // 2. Filter: Hibernated
  await page.evaluate(() => {
    const btns = Array.from(document.querySelectorAll('button'));
    const hibBtn = btns.find((b) => b.textContent && b.textContent.includes('Hibernated'));
    if (hibBtn) hibBtn.click();
  });
  await sleep(400);
  await snap('Filter Hibernated');

  // 3. Filter: Active
  await page.evaluate(() => {
    const btns = Array.from(document.querySelectorAll('button'));
    const actBtn = btns.find((b) => b.textContent && b.textContent.includes('Active'));
    if (actBtn) actBtn.click();
  });
  await sleep(400);
  await snap('Filter Active');

  // 4. Filter: All
  await page.evaluate(() => {
    const btns = Array.from(document.querySelectorAll('button'));
    const allBtn = btns.find((b) => b.textContent && b.textContent.includes('All'));
    if (allBtn) allBtn.click();
  });
  await sleep(400);
  await snap('Filter All');

  // 5. Open Simulator Modal
  await page.evaluate(() => {
    const btns = Array.from(document.querySelectorAll('button'));
    const simBtn = btns.find((b) => b.textContent && b.textContent.includes('Simulate Deploy'));
    if (simBtn) simBtn.click();
  });
  await sleep(500);
  await snap('Simulator Modal Open');

  // 6. Click Next.js Preset
  await page.evaluate(() => {
    const btns = Array.from(document.querySelectorAll('button'));
    const presetBtn = btns.find((b) => b.textContent && b.textContent.includes('Next.js'));
    if (presetBtn) presetBtn.click();
  });
  await sleep(400);
  await snap('Simulator Preset Selected');

  // 7. Close Simulator Modal
  await page.keyboard.press('Escape');
  await sleep(400);
  await snap('Dashboard Ready');

  // 8. Open Build Logs Drawer on PR #42
  await page.evaluate(() => {
    const btns = Array.from(document.querySelectorAll('button'));
    const logBtn = btns.find((b) => b.textContent && b.textContent.includes('Logs'));
    if (logBtn) logBtn.click();
  });
  await sleep(500);
  await snap('Build Logs Drawer Open');

  // 9. Filter Logs by "migration"
  const filterInput = await page.$('input[placeholder="Filter log lines..."]');
  if (filterInput) {
    await filterInput.type('migration', { delay: 40 });
    await sleep(400);
    await snap('Build Logs Filtered');
    await page.evaluate((el) => {
      el.value = '';
      el.dispatchEvent(new Event('input', { bubbles: true }));
    }, filterInput);
  }

  // 10. Switch to Container Exec tab
  await page.evaluate(() => {
    const btns = Array.from(document.querySelectorAll('button'));
    const execBtn = btns.find((b) => b.textContent && b.textContent.includes('Container Exec'));
    if (execBtn) execBtn.click();
  });
  await sleep(500);
  await snap('Container Exec Console Tab');

  // 11. Close Drawer
  await page.evaluate(() => {
    const btns = Array.from(document.querySelectorAll('button'));
    const closeBtn = btns.find((b) => b.querySelector('svg.lucide-x') || b.innerHTML.includes('lucide-x'));
    if (closeBtn) closeBtn.click();
  });
  await sleep(400);
  await snap('Back to Dashboard');

  await browser.close();
  console.log(`Captured ${frameIdx} frames. Generating optimized animated demo GIF with FFmpeg...`);

  // High quality 2-pass palettegen + paletteuse
  const cmd = `ffmpeg -y -framerate 1.2 -i "${path.join(FRAMES_DIR, 'frame_%02d.png')}" -vf "fps=1.2,scale=1080:-1:flags=lanczos,split[s0][s1];[s0]palettegen=max_colors=128[p];[s1][p]paletteuse=dither=bayer" -loop 0 "${OUTPUT_GIF}"`;
  execSync(cmd, { stdio: 'inherit' });

  const stats = fs.statSync(OUTPUT_GIF);
  console.log(`✓ Demo GIF successfully compiled: ${OUTPUT_GIF} (${(stats.size / 1024 / 1024).toFixed(2)} MB)`);
}

record().catch((err) => {
  console.error('Recording error:', err);
  process.exit(1);
});
