import { User as UserAlias } from './models/User';

function hello(name: string): void {
    console.log("Hello, " + name);
}

class Greeter {
    greet() { return "Hi"; }
}

class SuperUser extends UserAlias {
    role: string;
    constructor(id: string, name: string, role: string) {
        super(id, name);
        this.role = role;
    }
}

function main() {
    hello("world");
    const g = new Greeter();
    g.greet();
    
    const u = new UserAlias("1", "Alice");
    u.save();
}
