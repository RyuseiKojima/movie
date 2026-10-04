export interface Movie {
    id: number;
    title: string;
    overview?: string;
    release_date?: string;
    poster_path?: string | null;
    demo?: boolean;
}

export type WatchStatus = 'want' | 'watched';

export interface SavedMovie extends Movie {
    status: WatchStatus;
    rating: number;
    note: string;
}

export interface SearchResponse {
    results: Movie[];
    demo?: boolean;
}

export interface RecommendResponse {
    movie: Movie;
    confidence: number;
}

export interface ConfigResponse {
    demo: boolean;
    jev: boolean;
}

export type Tab = 'search' | 'library' | 'recommend';

export const isSameMovie = (a: Movie, b: Movie) => a.id === b.id && Boolean(a.demo) === Boolean(b.demo);
