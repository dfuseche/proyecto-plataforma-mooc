-- Tabla de Cursos (Agregado Raíz)
CREATE TABLE IF NOT EXISTS courses (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    slug VARCHAR(255) UNIQUE NOT NULL,
    title VARCHAR(255) NOT NULL,
    summary TEXT,
    created_by_teacher_id UUID NOT NULL REFERENCES users(id),
    current_published_version_id UUID,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_courses_teacher ON courses(created_by_teacher_id);
CREATE INDEX IF NOT EXISTS idx_courses_slug ON courses(slug);

-- Tabla de Versiones de Curso (Inmutables una vez publicadas)
CREATE TABLE IF NOT EXISTS course_versions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    course_id UUID NOT NULL REFERENCES courses(id) ON DELETE CASCADE,
    version_number INT NOT NULL,
    status VARCHAR(32) NOT NULL CHECK (status IN ('draft', 'published', 'archived')),
    passing_score NUMERIC(5,2) NOT NULL DEFAULT 70.00,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    published_at TIMESTAMP WITH TIME ZONE,
    UNIQUE(course_id, version_number)
);

CREATE INDEX IF NOT EXISTS idx_versions_course ON course_versions(course_id);

-- Tabla de Módulos (Jerarquía Nivel 2)
CREATE TABLE IF NOT EXISTS course_modules (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    version_id UUID NOT NULL REFERENCES course_versions(id) ON DELETE CASCADE,
    stable_id UUID NOT NULL DEFAULT uuid_generate_v4(),
    title VARCHAR(255) NOT NULL,
    description TEXT,
    position INT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_modules_version ON course_modules(version_id);
CREATE INDEX IF NOT EXISTS idx_modules_stable ON course_modules(stable_id);

-- Tabla de Unidades (Jerarquía Nivel 3)
CREATE TABLE IF NOT EXISTS course_units (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    module_id UUID NOT NULL REFERENCES course_modules(id) ON DELETE CASCADE,
    stable_id UUID NOT NULL DEFAULT uuid_generate_v4(),
    title VARCHAR(255) NOT NULL,
    position INT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_units_module ON course_units(module_id);
CREATE INDEX IF NOT EXISTS idx_units_stable ON course_units(stable_id);

-- Tabla de Recursos (Jerarquía Nivel 4)
CREATE TABLE IF NOT EXISTS course_resources (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    unit_id UUID NOT NULL REFERENCES course_units(id) ON DELETE CASCADE,
    stable_id UUID NOT NULL DEFAULT uuid_generate_v4(),
    title VARCHAR(255) NOT NULL,
    type VARCHAR(32) NOT NULL CHECK (type IN ('text', 'image', 'video', 'audio', 'pdf', 'presentation', 'download', 'iframe', 'link', 'quiz')),
    canonical_markdown TEXT,
    media_url VARCHAR(1024),
    is_visible BOOLEAN NOT NULL DEFAULT TRUE,
    is_mandatory BOOLEAN NOT NULL DEFAULT TRUE,
    is_downloadable BOOLEAN NOT NULL DEFAULT FALSE,
    position INT NOT NULL,
    processing_status VARCHAR(32) NOT NULL DEFAULT 'completed' CHECK (processing_status IN ('pending', 'processing', 'completed', 'failed')),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_resources_unit ON course_resources(unit_id);
CREATE INDEX IF NOT EXISTS idx_resources_stable ON course_resources(stable_id);
