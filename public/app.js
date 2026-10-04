const $ = selector => document.querySelector(selector);
let library;
try {
    library = JSON.parse(localStorage.getItem('movie-shelf') || '[]');
    if (!Array.isArray(library)) library = [];
} catch { library = []; }
let demo = false;

function notify(message) { $('#notice').textContent = message; }
async function api(path, options) {
    const response = await fetch(path, options);
    const data = await response.json();
    if (!response.ok) throw new Error(data.error || '通信に失敗しました。');
    return data;
}
function save() {
    try { localStorage.setItem('movie-shelf', JSON.stringify(library)); }
    catch { notify('保存できませんでした。ブラウザの保存容量をご確認ください。'); }
    renderLibrary();
}
function element(tag, text, className) {
    const node = document.createElement(tag);
    if (text !== undefined) node.textContent = text;
    if (className) node.className = className;
    return node;
}
function card(movie, saved = false) {
    const article = element('article', undefined, 'card');
    const poster = element('div', undefined, 'poster');
    if (movie.poster_path && /^\/[a-zA-Z0-9._-]+$/.test(movie.poster_path)) {
        const img = element('img');
        img.src = `https://image.tmdb.org/t/p/w342${movie.poster_path}`;
        img.alt = `${movie.title}のポスター`;
        img.loading = 'lazy';
        poster.append(img);
    } else poster.append(element('span', '◉'), element('p', movie.title));
    const content = element('div', undefined, 'card-content');
    content.append(element('p', `${movie.release_date?.slice(0, 4) || '公開年不明'}${movie.demo ? ' · デモ作品' : ''}`, 'muted'), element('h3', movie.title), element('p', movie.overview || 'あらすじはまだありません。', 'overview'));
    if (saved) {
        const status = element('select');
        status.setAttribute('aria-label', `${movie.title}の鑑賞状態`);
        for (const [value, text] of [['want', '観たい'], ['watched', '鑑賞済み']]) {
            const option = element('option', text);
            option.value = value;
            status.append(option);
        }
        status.value = movie.status;
        status.onchange = () => { movie.status = status.value; save(); };
        const rating = element('select');
        rating.setAttribute('aria-label', `${movie.title}の評価`);
        for (let i = 0; i <= 5; i++) {
            const option = element('option', i ? '★'.repeat(i) : '評価なし');
            option.value = i;
            rating.append(option);
        }
        rating.value = movie.rating || 0;
        rating.onchange = () => { movie.rating = Number(rating.value); save(); };
        const note = element('textarea');
        note.placeholder = '感想や覚えておきたいこと';
        note.setAttribute('aria-label', `${movie.title}のメモ`);
        note.maxLength = 2000;
        note.value = movie.note || '';
        note.onchange = () => { movie.note = note.value; save(); };
        const remove = element('button', '棚から削除', 'remove');
        remove.onclick = () => {
            if (!confirm(`「${movie.title}」を映画棚から削除しますか？`)) return;
            library = library.filter(item => item !== movie);
            save();
        };
        content.append(status, rating, note, remove);
    } else {
        const exists = library.some(item => item.id === movie.id && Boolean(item.demo) === Boolean(movie.demo));
        const button = element('button', exists ? '✓ 登録済み' : '＋ 観たい映画に登録', 'add');
        button.disabled = exists;
        button.onclick = () => {
            library.push({ ...movie, status: 'want', rating: 0, note: '' });
            save();
            button.disabled = true;
            button.textContent = '✓ 登録済み';
            notify(`「${movie.title}」を登録しました。`);
        };
        content.append(button);
    }
    article.append(poster, content);
    return article;
}
function renderLibrary() {
    $('#count').textContent = library.length;
    const items = library.filter(movie => $('#filter').value === 'all' || movie.status === $('#filter').value);
    $('#library').replaceChildren(...items.map(movie => card(movie, true)));
    if (!items.length) $('#library').append(element('p', 'まだ映画がありません。気になる一本を検索して登録しましょう。', 'empty'));
}
async function search() {
    const button = $('#search-form button');
    button.disabled = true;
    notify('映画を探しています…');
    try {
        const data = await api(`/api/search?q=${encodeURIComponent($('#query').value)}`);
        $('#search-title').textContent = $('#query').value.trim() ? '検索結果' : 'いま注目の映画';
        $('#results').replaceChildren(...data.results.map(movie => card({ ...movie, demo: Boolean(data.demo) })));
        notify(data.demo ? 'デモモード：架空のサンプル作品を表示しています。実際の映画検索にはTMDBのAPIキーを設定してください。' : `${data.results.length}件の映画が見つかりました。`);
    } catch (error) { notify(error.message); }
    finally { button.disabled = false; }
}
for (const button of document.querySelectorAll('[data-tab]')) button.onclick = () => {
    for (const item of document.querySelectorAll('[data-tab]')) item.classList.toggle('active', item === button);
    for (const name of ['search', 'library', 'recommend']) $(`#${name}-panel`).hidden = name !== button.dataset.tab;
    notify(demo ? 'デモモード：実際の検索・提案にはAPIキーを設定してください。' : '');
    renderLibrary();
};
$('#search-form').onsubmit = event => { event.preventDefault(); search(); };
$('#filter').onchange = renderLibrary;
$('#recommend-form').onsubmit = async event => {
    event.preventDefault();
    const button = $('#recommend-form button');
    button.disabled = true;
    $('#suggestion').replaceChildren();
    notify('Jevが次の一本を選んでいます…');
    try {
        const result = await api('/api/recommend', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ mood: $('#mood').value, library }) });
        $('#suggestion').append(card(result.movie));
        notify(`Jevが選んだ一本です。モデルの確信度：${Math.round(result.confidence * 100)}%（満足度の保証ではありません）`);
    } catch (error) { notify(error.message); }
    finally { button.disabled = false; }
};
renderLibrary();
api('/api/config').then(config => { demo = config.demo; search(); }).catch(error => notify(error.message));
