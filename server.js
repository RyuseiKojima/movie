import http from 'node:http';
import { readFile } from 'node:fs/promises';
import { extname } from 'node:path';
import { fileURLToPath } from 'node:url';

const distDir = new URL('./dist/', import.meta.url);
const staticTypes = { '.html': 'text/html', '.js': 'text/javascript', '.css': 'text/css', '.svg': 'image/svg+xml' };

export const demoMovies = [
    { id: 1, title: '星をめぐる旅', overview: '遠い星を目指す探検家と、地球で待つ家族の物語。', release_date: '2024-01-01', genre_ids: [878], vote_average: 8.2 },
    { id: 2, title: '雨の日の喫茶店', overview: '小さな喫茶店で出会った二人が、少しずつ新しい日常を見つける。', release_date: '2023-01-01', genre_ids: [18], vote_average: 7.8 },
    { id: 3, title: '真夜中の手紙', overview: '届くはずのない手紙を手がかりに、探偵が街の秘密を追う。', release_date: '2025-01-01', genre_ids: [9648], vote_average: 7.5 }
];

async function requestJson(url, options = {}) {
    const response = await fetch(url, { ...options, signal: AbortSignal.timeout(15000) });
    if (!response.ok) throw new Error('外部APIへの接続に失敗しました。APIキーと利用状況をご確認ください。');
    return response.json();
}

async function tmdb(path, params = {}) {
    const url = new URL(`https://api.themoviedb.org/3/${path}`);
    for (const [key, value] of Object.entries({ language: 'ja-JP', include_adult: 'false', ...params })) url.searchParams.set(key, value);
    const credential = (process.env.TMDB_ACCESS_TOKEN || '').trim();
    if (/^[a-f0-9]{32}$/i.test(credential)) {
        url.searchParams.set('api_key', credential);
        return requestJson(url);
    }
    return requestJson(url, { headers: { Authorization: `Bearer ${credential}` } });
}

export async function recommend({ mood, library }, fetcher = requestJson) {
    if (typeof mood !== 'string' || mood.length > 500 || !Array.isArray(library) || library.length > 500) throw new Error('気分または登録作品の形式が不正です。');
    if (!process.env.TMDB_ACCESS_TOKEN || !process.env.TYPESAFE_API_KEY) throw new Error('提案にはTMDBとJevのAPIキーが必要です。.envを設定してください。');
    const watched = library.filter(movie => movie.status === 'watched');
    const favorite = watched.filter(movie => movie.rating >= 4).sort((a, b) => b.rating - a.rating)[0];
    const data = favorite && !favorite.demo
        ? await tmdb(`movie/${Number(favorite.id)}/recommendations`)
        : await tmdb('movie/popular');
    const candidates = data.results.filter(movie => !library.some(saved => !saved.demo && saved.id === movie.id)).slice(0, 15);
    if (!candidates.length) throw new Error('候補が見つかりませんでした。別の作品を評価してお試しください。');
    const result = await fetcher('https://api.typesafe.ai/v1/systemone', {
        method: 'POST',
        headers: { Authorization: `Bearer ${process.env.TYPESAFE_API_KEY}`, 'Content-Type': 'application/json' },
        body: JSON.stringify({
            model: process.env.JEV_MODEL || 'jev-latest',
            state: { mood, watched: watched.map(({ title, rating, note }) => ({ title, rating, note })) },
            questions: { movie: {
                type: 'choice',
                instructions: '鑑賞履歴、評価、メモと今の気分に合う映画を1本選んでください。合う候補がない場合はnoneを選んでください。',
                criteria: { ...Object.fromEntries(candidates.map(movie => [String(movie.id), { title: movie.title, overview: movie.overview }])), none: '合う映画がない' }
            } }
        })
    });
    const answer = result.answers?.movie;
    const movie = candidates.find(candidate => String(candidate.id) === answer?.choice);
    if (!movie) throw new Error('気分に合う作品を選べませんでした。もう少し具体的な気分を入力してください。');
    return { movie, confidence: answer.confidence };
}

export const server = http.createServer(async (req, res) => {
    const url = new URL(req.url, 'http://localhost');
    const json = (status, data) => {
        res.writeHead(status, { 'Content-Type': 'application/json; charset=utf-8', 'Cache-Control': 'no-store' });
        res.end(JSON.stringify(data));
    };
    try {
        if (url.pathname === '/api/config') return json(200, { demo: !process.env.TMDB_ACCESS_TOKEN, jev: Boolean(process.env.TYPESAFE_API_KEY) });
        if (url.pathname === '/api/search') {
            const query = (url.searchParams.get('q') || '').trim();
            if (query.length > 200) return json(400, { error: '検索語は200文字以内で入力してください。' });
            if (!process.env.TMDB_ACCESS_TOKEN) return json(200, { results: demoMovies.filter(movie => !query || movie.title.includes(query)), demo: true });
            return json(200, await tmdb(query ? 'search/movie' : 'movie/popular', query ? { query } : {}));
        }
        if (url.pathname === '/api/recommend' && req.method === 'POST') {
            let body = '';
            for await (const chunk of req) {
                body += chunk;
                if (Buffer.byteLength(body) > 100000) return json(413, { error: '登録データが大きすぎます。' });
            }
            return json(200, await recommend(JSON.parse(body)));
        }
        const name = url.pathname === '/' ? 'index.html' : url.pathname.slice(1);
        const file = new URL(name, distDir);
        const type = staticTypes[extname(name)];
        if (!type || !file.href.startsWith(distDir.href)) return json(404, { error: '見つかりません。' });
        let content;
        try { content = await readFile(file); }
        catch { return json(404, { error: '見つかりません。' }); }
        res.writeHead(200, { 'Content-Type': `${type}; charset=utf-8`, 'X-Content-Type-Options': 'nosniff' });
        res.end(content);
    } catch (error) {
        json(400, { error: error instanceof SyntaxError ? '入力形式が不正です。' : error.message });
    }
});

if (process.argv[1] === fileURLToPath(import.meta.url)) {
    server.listen(Number(process.env.PORT || 3000), '127.0.0.1', () => console.log(`Movie Shelf: http://localhost:${process.env.PORT || 3000}`));
}
