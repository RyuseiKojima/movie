import test from 'node:test';
import assert from 'node:assert/strict';
import { server, recommend } from '../server.js';

test('デモ検索、静的配信、API入力検証', async () => {
    const token = process.env.TMDB_ACCESS_TOKEN;
    delete process.env.TMDB_ACCESS_TOKEN;
    await new Promise(resolve => server.listen(0, '127.0.0.1', resolve));
    const base = `http://127.0.0.1:${server.address().port}`;
    try {
        const search = await fetch(`${base}/api/search?q=${encodeURIComponent('喫茶店')}`).then(response => response.json());
        assert.equal(search.demo, true);
        assert.equal(search.results.length, 1);
        assert.equal(search.results[0].id, 2);
        const home = await fetch(base);
        assert.equal(home.status, 200);
        assert.match(await home.text(), /Movie Shelf/);
        assert.equal((await fetch(`${base}/unknown`)).status, 404);
        const invalid = await fetch(`${base}/api/recommend`, { method: 'POST', body: '{' });
        assert.equal(invalid.status, 400);
        const oversized = await fetch(`${base}/api/recommend`, { method: 'POST', body: 'x'.repeat(100001) });
        assert.equal(oversized.status, 413);
    } finally {
        await new Promise(resolve => server.close(resolve));
        if (token !== undefined) process.env.TMDB_ACCESS_TOKEN = token;
    }
});

test('Jevには未登録候補だけを渡し、候補外の応答を拒否する', async () => {
    const originalFetch = globalThis.fetch;
    const previous = { token: process.env.TMDB_ACCESS_TOKEN, key: process.env.TYPESAFE_API_KEY };
    process.env.TMDB_ACCESS_TOKEN = 'test-token';
    process.env.TYPESAFE_API_KEY = 'test-key';
    globalThis.fetch = async () => ({ ok: true, json: async () => ({ results: [{ id: 10, title: '登録済み' }, { id: 20, title: '未登録', overview: '冒険映画' }] }) });
    const input = { mood: '冒険したい', library: [{ id: 10, title: '登録済み', status: 'watched', rating: 3 }] };
    try {
        const result = await recommend(input, async (url, options) => {
            assert.equal(url, 'https://api.typesafe.ai/v1/systemone');
            const request = JSON.parse(options.body);
            assert.equal(request.questions.movie.criteria['10'], undefined);
            assert.equal(request.questions.movie.criteria['20'].title, '未登録');
            assert.ok(request.questions.movie.criteria.none);
            return { answers: { movie: { choice: '20', confidence: 0.8 } } };
        });
        assert.equal(result.movie.id, 20);
        await assert.rejects(recommend(input, async () => ({ answers: { movie: { choice: '999' } } })), /選べません/);
        await assert.rejects(recommend({ mood: 'x', library: null }), /形式が不正/);
    } finally {
        globalThis.fetch = originalFetch;
        for (const [name, value] of [['TMDB_ACCESS_TOKEN', previous.token], ['TYPESAFE_API_KEY', previous.key]]) {
            if (value === undefined) delete process.env[name];
            else process.env[name] = value;
        }
    }
});
