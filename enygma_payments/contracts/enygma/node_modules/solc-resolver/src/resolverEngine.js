module.exports = class ResolverEngine {
  constructor() {
    this.resolvers = [];
    this.previouslyHandled = {};
  }

  async getContent(url) {
    for (let resolve of this.resolvers) {
      if (this.getResolverType(url) == resolve.type) {
        const result = await resolve.parser(url);
        if (result) return result;
      }
    }
    return;
  }

  // get data
  async require(importPath) {
    const imported = this.previouslyHandled[importPath];
    // get source from cache
    if (imported) {
      let result = this.getResultFromImported(imported, importPath);
      return result.content;
    }

    const handlerType = this.getResolverType(importPath);
    const content = await this.getContent(importPath);

    this.previouslyHandled[importPath] = {
      content: content,
      type: handlerType,
      importPath
    };

    return content;
  }

  // chain pattern
  addResolver(resolver) {
    this.resolvers.push(resolver);
    return this;
  }

  getResultFromImported(imported, importPath) {
    return {
      content: imported.content,
      type: imported.type,
      importPath
    };
  }

  getResolverType(url) {
    for (let resolver of this.resolvers) {
      let match = resolver.match.exec(url);
      if (match) {
        return resolver.type;
      }
    }
    return;
  }

  isMatch(importPath) {
    let found = false;
    if (this.resolvers.length == 0) return false;
    for (let resolver of this.resolvers) {
      if (found) break;
      const match = resolver.match.exec(importPath);
      if (match) found = true;
    }
    return !found;
  }
};