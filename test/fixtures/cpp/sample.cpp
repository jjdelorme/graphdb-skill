#include "math.h"
#include <iostream>

int global_counter = 0;

class Base {
public:
    int id;
};

class Derived : public Base {
public:
    void doWork() {
        global_counter++;
        id = 100;
        int result = Math::Add(5, 10);
    }
};

void main() {
    Derived d;
    d.doWork();
}
