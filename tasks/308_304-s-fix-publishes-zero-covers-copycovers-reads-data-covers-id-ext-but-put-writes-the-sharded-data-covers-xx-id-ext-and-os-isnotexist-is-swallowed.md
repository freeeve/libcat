# 308 -- 304's fix publishes zero covers: copyCovers reads data/covers/<id>.ext but PUT writes the sharded data/covers/<xx>/<id>.ext, and os.IsNotExist is swallowed

Filed from libcat on 2026-07-10 (cross-repo ask).
